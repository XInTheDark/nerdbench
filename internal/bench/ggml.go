package bench

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"runtime"
	"strconv"
	"time"

	"github.com/nerdbench/nerdbench/internal/assets"
	"github.com/nerdbench/nerdbench/internal/runner"
)

func (b Benchmark) runGGML(ctx context.Context, req RunRequest) (RunOutput, bool, error) {
	asset, ok := assets.FindWorker("ggml-ml-kernel", runtime.GOOS, runtime.GOARCH)
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

	threads := req.Threads
	if req.Mode == ModeSingle {
		threads = 1
	}

	iterations := 10
	if req.Profile != "smoke" {
		calibration, err := runGGMLIterations(ctx, worker.Path, threads, iterations)
		if err != nil {
			return RunOutput{
				Duration:   calibration.duration,
				StdoutTail: runner.Tail(calibration.stdout, 4096),
				StderrTail: runner.Tail(calibration.stderr, 4096),
			}, true, err
		}
		if calibration.duration > 0 {
			iterations = maxInt(iterations, int(math.Ceil(float64(iterations)*profileDuration(req.Profile).Seconds()/calibration.duration.Seconds())))
		}
	}

	measured, err := runGGMLIterations(ctx, worker.Path, threads, iterations)
	if err != nil {
		return RunOutput{
			Duration:   measured.duration,
			StdoutTail: runner.Tail(measured.stdout, 4096),
			StderrTail: runner.Tail(measured.stderr, 4096),
		}, true, err
	}
	target := profileDuration(req.Profile)
	if req.Profile != "smoke" && measured.duration > 0 && measured.duration < target*8/10 {
		iterations = maxInt(iterations+1, int(math.Ceil(float64(iterations)*target.Seconds()/measured.duration.Seconds())))
		measured, err = runGGMLIterations(ctx, worker.Path, threads, iterations)
		if err != nil {
			return RunOutput{
				Duration:   measured.duration,
				StdoutTail: runner.Tail(measured.stdout, 4096),
				StderrTail: runner.Tail(measured.stderr, 4096),
			}, true, err
		}
	}

	value := parseGGMLResult(measured.stdout + "\n" + measured.stderr)
	if value <= 0 {
		return RunOutput{
			Duration:   measured.duration,
			StdoutTail: runner.Tail(measured.stdout, 4096),
			StderrTail: runner.Tail(measured.stderr, 4096),
		}, true, fmt.Errorf("ggml-ml-kernel: could not parse result")
	}

	return RunOutput{
		Value:      value,
		Duration:   measured.duration,
		Iterations: uint64(iterations),
		StdoutTail: runner.Tail(measured.stdout, 256),
		StderrTail: runner.Tail(measured.stderr, 512),
		Note:       "ggml-ml-kernel embedded worker",
	}, true, nil
}

type ggmlRunResult struct {
	duration time.Duration
	stdout   string
	stderr   string
}

func runGGMLIterations(ctx context.Context, workerPath string, threads, iterations int) (ggmlRunResult, error) {
	proc, err := runner.RunProcess(
		ctx,
		workerPath,
		"--threads", strconv.Itoa(threads),
		"--iter", strconv.Itoa(iterations),
	)
	return ggmlRunResult{
		duration: proc.Duration,
		stdout:   proc.Stdout,
		stderr:   proc.Stderr,
	}, err
}

var ggmlResultRE = regexp.MustCompile(`NERDBENCH_GGML_RESULT:([0-9.]+)`)

func parseGGMLResult(log string) float64 {
	matches := ggmlResultRE.FindStringSubmatch(log)
	if len(matches) != 2 {
		return 0
	}
	v, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0
	}
	return v
}
