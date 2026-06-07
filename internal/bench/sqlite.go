package bench

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nerdbench/nerdbench/internal/assets"
	"github.com/nerdbench/nerdbench/internal/runner"
)

func (b Benchmark) runSQLite(ctx context.Context, req RunRequest) (RunOutput, bool, error) {
	asset, ok := assets.FindWorker("sqlite-speedtest", runtime.GOOS, runtime.GOARCH)
	if !ok {
		return RunOutput{}, false, nil
	}
	worker, err := runner.ExtractWorker(runner.ExtractRequest{
		Name:      asset.Name,
		Bytes:     asset.Bytes,
		SHA256Hex: asset.SHA256,
	})
	if err != nil {
		return RunOutput{}, true, err
	}
	defer worker.Cleanup()

	workers := req.Threads
	if req.Mode == ModeSingle {
		workers = 1
	}
	size := sqliteSize(req.Profile)
	if req.Profile != "smoke" {
		measured, err := runSQLiteTimed(ctx, worker.Path, workers, size, profileDuration(req.Profile))
		if err != nil {
			return RunOutput{
				Duration:   measured.elapsed,
				StdoutTail: runner.Tail(measured.stdout, 4096),
				StderrTail: runner.Tail(measured.stderr, 4096),
			}, true, err
		}
		if measured.completed == 0 || measured.elapsed <= 0 {
			return RunOutput{
				Duration:   measured.elapsed,
				StdoutTail: runner.Tail(measured.stdout, 4096),
				StderrTail: runner.Tail(measured.stderr, 4096),
			}, true, fmt.Errorf("sqlite-speedtest: no completed runs")
		}
		value := float64(measured.completed) / measured.elapsed.Seconds()
		return RunOutput{
			Value:      value,
			Duration:   measured.elapsed,
			Iterations: uint64(measured.completed),
			StdoutTail: runner.Tail(measured.stdout, 256),
			StderrTail: runner.Tail(measured.stderr, 512),
			Note:       "sqlite-speedtest embedded worker",
		}, true, nil
	}

	measured, err := runSQLiteWorkers(ctx, worker.Path, workers, size, 1)
	if err != nil {
		return RunOutput{
			Duration:   measured.elapsed,
			StdoutTail: runner.Tail(measured.stdout, 4096),
			StderrTail: runner.Tail(measured.stderr, 4096),
		}, true, err
	}
	if measured.maxSeconds <= 0 {
		return RunOutput{
			Duration:   measured.elapsed,
			StdoutTail: runner.Tail(measured.stdout, 4096),
			StderrTail: runner.Tail(measured.stderr, 4096),
		}, true, fmt.Errorf("sqlite-speedtest TOTAL line not found")
	}
	value := float64(workers) / measured.maxSeconds
	return RunOutput{
		Value:      value,
		Duration:   measured.elapsed,
		Iterations: uint64(workers),
		StdoutTail: runner.Tail(measured.stdout, 256),
		StderrTail: runner.Tail(measured.stderr, 512),
		Note:       "sqlite-speedtest embedded worker",
	}, true, nil
}

type sqliteRunResult struct {
	maxSeconds float64
	completed  int
	elapsed    time.Duration
	stdout     string
	stderr     string
}

func runSQLiteTimed(ctx context.Context, workerPath string, workers, size int, target time.Duration) (sqliteRunResult, error) {
	start := time.Now()
	deadline := start.Add(target)
	var stdoutAll, stderrAll string
	var completed uint64
	var stopped int32
	var firstErr error
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			dir := filepath.Join(filepath.Dir(workerPath), fmt.Sprintf("sqlite-%03d", index))
			if err := os.MkdirAll(dir, 0o700); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				atomic.StoreInt32(&stopped, 1)
				return
			}
			for time.Now().Before(deadline) && atomic.LoadInt32(&stopped) == 0 {
				proc, err := runner.RunProcessInDir(
					ctx,
					dir,
					workerPath,
					"--memdb",
					"--singlethread",
					"--size", strconv.Itoa(size),
					"--repeat", "1",
					"--testset", "main",
				)
				if proc.Stdout != "" || proc.Stderr != "" {
					mu.Lock()
					stdoutAll += proc.Stdout
					stderrAll += proc.Stderr
					stdoutAll = runner.Tail(stdoutAll, 4096)
					stderrAll = runner.Tail(stderrAll, 4096)
					mu.Unlock()
				}
				if err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					mu.Unlock()
					atomic.StoreInt32(&stopped, 1)
					return
				}
				if parseSQLiteTotalSeconds(proc.Stdout+"\n"+proc.Stderr) <= 0 {
					mu.Lock()
					if firstErr == nil {
						firstErr = fmt.Errorf("sqlite-speedtest TOTAL line not found")
					}
					mu.Unlock()
					atomic.StoreInt32(&stopped, 1)
					return
				}
				atomic.AddUint64(&completed, 1)
			}
		}(i)
	}
	wg.Wait()

	return sqliteRunResult{
		completed: int(completed),
		elapsed:   time.Since(start),
		stdout:    stdoutAll,
		stderr:    stderrAll,
	}, firstErr
}

func runSQLiteWorkers(ctx context.Context, workerPath string, workers, size, repeat int) (sqliteRunResult, error) {
	start := time.Now()
	type oneRun struct {
		seconds float64
		stdout  string
		stderr  string
		err     error
	}
	runs := make([]oneRun, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			dir := filepath.Join(filepath.Dir(workerPath), fmt.Sprintf("sqlite-%03d", index))
			if err := os.MkdirAll(dir, 0o700); err != nil {
				runs[index].err = err
				return
			}
			proc, err := runner.RunProcessInDir(
				ctx,
				dir,
				workerPath,
				"--memdb",
				"--singlethread",
				"--size", strconv.Itoa(size),
				"--repeat", strconv.Itoa(repeat),
				"--testset", "main",
			)
			runs[index].stdout = proc.Stdout
			runs[index].stderr = proc.Stderr
			runs[index].seconds = parseSQLiteTotalSeconds(proc.Stdout + "\n" + proc.Stderr)
			runs[index].err = err
		}(i)
	}
	wg.Wait()
	elapsed := time.Since(start)

	var maxSeconds float64
	var stdoutTail string
	var stderrTail string
	for _, run := range runs {
		stdoutTail += run.stdout
		stderrTail += run.stderr
		if run.err != nil {
			return sqliteRunResult{
				elapsed: elapsed,
				stdout:  stdoutTail,
				stderr:  stderrTail,
			}, run.err
		}
		if run.seconds <= 0 {
			return sqliteRunResult{
				elapsed: elapsed,
				stdout:  stdoutTail,
				stderr:  stderrTail,
			}, nil
		}
		if run.seconds > maxSeconds {
			maxSeconds = run.seconds
		}
	}
	return sqliteRunResult{
		maxSeconds: maxSeconds,
		elapsed:    elapsed,
		stdout:     stdoutTail,
		stderr:     stderrTail,
	}, nil
}

func sqliteSize(profile string) int {
	switch profile {
	case "smoke":
		return 1
	case "quick":
		return 20
	case "extended":
		return 150
	default:
		return 80
	}
}

var sqliteTotalRE = regexp.MustCompile(`TOTAL\.+\s+([0-9.]+)s`)

func parseSQLiteTotalSeconds(log string) float64 {
	matches := sqliteTotalRE.FindStringSubmatch(log)
	if len(matches) != 2 {
		return 0
	}
	value, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0
	}
	return value
}
