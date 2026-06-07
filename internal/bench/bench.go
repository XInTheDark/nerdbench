package bench

import (
	"context"
	"crypto/sha256"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nerdbench/nerdbench/internal/assets"
	"github.com/nerdbench/nerdbench/internal/results"
	"github.com/nerdbench/nerdbench/internal/runner"
)

type Mode string

const (
	ModeSingle Mode = "single"
	ModeMulti  Mode = "multi"
)

type Direction string

const (
	HigherIsBetter Direction = "higher"
	LowerIsBetter  Direction = "lower"
)

type Benchmark struct {
	Name       string
	MetricName string
	Unit       string
	Direction  Direction
	Source     string
	Revision   string
	License    string
	WorkerHash string
	Compiler   string
	BuildFlags string
	Command    string
	Seed       uint64
	Weight     float64
}

type RunRequest struct {
	Profile string
	Mode    Mode
	Threads int
}

type RunOutput struct {
	Value      float64
	Duration   time.Duration
	Iterations uint64
	StdoutTail string
	StderrTail string
	Note       string
}

func (b Benchmark) Run(ctx context.Context, req RunRequest) (RunOutput, error) {
	if b.Name == "c-ray" {
		if out, ok, err := b.runCRay(ctx, req); ok || err != nil {
			return out, err
		}
	}
	if b.Name == "sqlite-speedtest" {
		if out, ok, err := b.runSQLite(ctx, req); ok || err != nil {
			return out, err
		}
	}
	if b.Name == "sysbench" {
		if out, ok, err := b.runSysbench(ctx, req); ok || err != nil {
			return out, err
		}
	}
	if b.Name == "stockfish" {
		if out, ok, err := b.runStockfish(ctx, req); ok || err != nil {
			return out, err
		}
	}
	if b.Name == "openssl-speed" {
		if out, ok, err := b.runOpenSSL(ctx, req); ok || err != nil {
			return out, err
		}
	}
	if b.Name == "zstd" {
		if out, ok, err := b.runZstd(ctx, req); ok || err != nil {
			return out, err
		}
	}
	if b.Name == "ggml-ml-kernel" {
		if out, ok, err := b.runGGML(ctx, req); ok || err != nil {
			return out, err
		}
	}
	if b.Name == "tinycc-compile" {
		if out, ok, err := b.runTinyCC(ctx, req); ok || err != nil {
			return out, err
		}
	}
	return RunOutput{}, fmt.Errorf("no embedded worker for %s on %s/%s", b.Name, runtime.GOOS, runtime.GOARCH)
}

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
			dir := filepath.Join(filepath.Dir(worker.Path), fmt.Sprintf("sqlite-%03d", index))
			if err := os.MkdirAll(dir, 0o700); err != nil {
				runs[index].err = err
				return
			}
			proc, err := runner.RunProcessInDir(
				ctx,
				dir,
				worker.Path,
				"--memdb",
				"--singlethread",
				"--size", strconv.Itoa(size),
				"--repeat", "1",
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
			return RunOutput{
				Duration:   elapsed,
				StdoutTail: runner.Tail(stdoutTail, 4096),
				StderrTail: runner.Tail(stderrTail, 4096),
			}, true, run.err
		}
		if run.seconds <= 0 {
			return RunOutput{
				Duration:   elapsed,
				StdoutTail: runner.Tail(stdoutTail, 4096),
				StderrTail: runner.Tail(stderrTail, 4096),
			}, true, fmt.Errorf("sqlite-speedtest TOTAL line not found")
		}
		if run.seconds > maxSeconds {
			maxSeconds = run.seconds
		}
	}
	value := float64(workers) / maxSeconds
	return RunOutput{
		Value:      value,
		Duration:   elapsed,
		Iterations: uint64(workers),
		StdoutTail: runner.Tail(stdoutTail, 256),
		StderrTail: runner.Tail(stderrTail, 512),
		Note:       "sqlite-speedtest embedded worker",
	}, true, nil
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

func (b Benchmark) runCRay(ctx context.Context, req RunRequest) (RunOutput, bool, error) {
	asset, ok := assets.FindWorker("c-ray", runtime.GOOS, runtime.GOARCH)
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
	width, height, samples, bounces := cRaySceneParams(req.Profile)
	scene := cRayScene(width, height, samples, bounces, threads)
	scenePath := filepath.Join(filepath.Dir(worker.Path), "scene.json")
	if err := os.MkdirAll(filepath.Join(filepath.Dir(worker.Path), "output"), 0o700); err != nil {
		return RunOutput{}, true, err
	}
	if err := os.WriteFile(scenePath, []byte(scene), 0o600); err != nil {
		return RunOutput{}, true, err
	}

	start := time.Now()
	workDir := filepath.Dir(worker.Path)
	proc, err := runner.RunProcessInDir(ctx, workDir, worker.Path, scenePath)
	elapsed := time.Since(start)
	if err != nil {
		return RunOutput{
			Duration:   elapsed,
			StdoutTail: runner.Tail(proc.Stdout, 4096),
			StderrTail: runner.Tail(proc.Stderr, 4096),
		}, true, err
	}
	renderSeconds := parseCRaySeconds(proc.Stderr)
	if renderSeconds <= 0 {
		renderSeconds = proc.Duration.Seconds()
	}
	paths := float64(width * height * samples)
	value := paths / renderSeconds
	return RunOutput{
		Value:      value,
		Duration:   elapsed,
		Iterations: uint64(paths),
		StdoutTail: runner.Tail(proc.Stdout, 512),
		StderrTail: runner.Tail(proc.Stderr, 512),
		Note:       "c-ray embedded worker",
	}, true, nil
}

func cRaySceneParams(profile string) (width, height, samples, bounces int) {
	switch profile {
	case "smoke":
		return 128, 128, 4, 4
	case "quick":
		return 256, 256, 12, 6
	case "extended":
		return 512, 512, 32, 8
	default:
		return 384, 384, 24, 8
	}
}

func cRayScene(width, height, samples, bounces, threads int) string {
	return fmt.Sprintf(`{
  "version": 1.0,
  "renderer": {
    "threads": %d,
    "samples": %d,
    "bounces": %d,
    "tileWidth": 16,
    "tileHeight": 16,
    "tileOrder": "fromMiddle",
    "outputFilePath": "output/",
    "outputFileName": "rendered",
    "fileType": "png",
    "count": 0,
    "width": %d,
    "height": %d
  },
  "display": { "isFullscreen": false, "windowScale": 1.0 },
  "camera": {
    "FOV": 30.0,
    "focalDistance": 0.7,
    "fstops": 0,
    "transforms": [
      { "type": "translate", "x": 0, "y": 0, "z": -1.5 }
    ]
  },
  "scene": {
    "ambientColor": {
      "type": "background",
      "down": { "r": 1.0, "g": 1.0, "b": 1.0 },
      "up": { "r": 0.5, "g": 0.7, "b": 1.0 }
    },
    "primitives": [
      {
        "type": "sphere",
        "instances": [{ "transforms": [{ "type": "translate", "x": 0, "y": 0, "z": 0 }] }],
        "material": { "type": "diffuse", "color": { "r": 0.9, "g": 0.2, "b": 0.2 } },
        "radius": 0.25
      }
    ],
    "meshes": []
  }
}
`, threads, samples, bounces, width, height)
}

var cRayFinishedRE = regexp.MustCompile(`Finished render in ([0-9.]+)(ms|s)`)

func parseCRaySeconds(log string) float64 {
	matches := cRayFinishedRE.FindStringSubmatch(log)
	if len(matches) != 3 {
		return 0
	}
	value, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0
	}
	if matches[2] == "ms" {
		return value / 1000
	}
	return value
}

func (b Benchmark) runSysbench(ctx context.Context, req RunRequest) (RunOutput, bool, error) {
	asset, ok := assets.FindWorker("sysbench", runtime.GOOS, runtime.GOARCH)
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
	limit := sysbenchCPUSeconds(req.Profile)
	var stdoutAll, stderrAll string
	start := time.Now()

	args := []string{"cpu", "--time=" + fmt.Sprintf("%d", limit), "--threads=" + fmt.Sprintf("%d", threads), "run"}
	proc, err := runner.RunProcess(ctx, worker.Path, args...)
	stdoutAll += proc.Stdout
	stderrAll += proc.Stderr
	elapsed := time.Since(start)
	if err != nil {
		return RunOutput{
			Duration:   elapsed,
			StdoutTail: runner.Tail(stdoutAll, 4096),
			StderrTail: runner.Tail(stderrAll, 4096),
		}, true, err
	}
	value := parseSysbenchEPS(proc.Stdout + "\n" + proc.Stderr)
	if value <= 0 {
		return RunOutput{
			Duration:   elapsed,
			StdoutTail: runner.Tail(stdoutAll, 4096),
			StderrTail: runner.Tail(stderrAll, 4096),
		}, true, fmt.Errorf("sysbench: events per second not found")
	}

	return RunOutput{
		Value:      value,
		Duration:   elapsed,
		Iterations: 0,
		StdoutTail: runner.Tail(stdoutAll, 256),
		StderrTail: runner.Tail(stderrAll, 512),
		Note:       "sysbench embedded worker",
	}, true, nil
}

func sysbenchCPUSeconds(profile string) int {
	switch profile {
	case "smoke":
		return 1
	case "quick":
		return 3
	case "extended":
		return 20
	default:
		return 8
	}
}

var sysbenchEPSRE = regexp.MustCompile(`events per second:\s*([0-9.]+)`)

func parseSysbenchEPS(log string) float64 {
	matches := sysbenchEPSRE.FindStringSubmatch(log)
	if len(matches) != 2 {
		return 0
	}
	v, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0
	}
	return v
}

func (b Benchmark) runStockfish(ctx context.Context, req RunRequest) (RunOutput, bool, error) {
	asset, ok := assets.FindWorker("stockfish", runtime.GOOS, runtime.GOARCH)
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
	runs := stockfishRuns(req.Profile)
	var allNPS []float64
	var stdoutAll, stderrAll string

	for i := 0; i < runs; i++ {
		// Stockfish reads from stdin; use shell echo to pipe commands
		input := fmt.Sprintf("uci\nsetoption name Threads value %d\nbench\nquit\n", threads)
		shellCmd := fmt.Sprintf("echo '%s' | '%s'", input, worker.Path)
		proc, err := runner.RunProcess(ctx, "/bin/sh", "-c", shellCmd)
		stdoutAll += proc.Stdout
		stderrAll += proc.Stderr
		if err != nil {
			continue
		}
		nps := parseStockfishNPS(proc.Stdout + "\n" + proc.Stderr)
		if nps > 0 {
			allNPS = append(allNPS, nps)
		}
	}

	if len(allNPS) == 0 {
		return RunOutput{
			StdoutTail: runner.Tail(stdoutAll, 4096),
			StderrTail: runner.Tail(stderrAll, 4096),
		}, true, fmt.Errorf("stockfish: could not parse nodes/second")
	}

	median := medianFloat64(allNPS)
	return RunOutput{
		Value:      median,
		Duration:   time.Duration(0),
		Iterations: 0,
		StdoutTail: runner.Tail(stdoutAll, 256),
		StderrTail: runner.Tail(stderrAll, 512),
		Note:       "stockfish embedded worker",
	}, true, nil
}

func stockfishRuns(profile string) int {
	switch profile {
	case "smoke":
		return 1
	case "quick":
		return 2
	case "extended":
		return 5
	default:
		return 3
	}
}

var stockfishNPSRE = regexp.MustCompile(`Nodes/second\s*:\s*([0-9.]+)`)

func parseStockfishNPS(log string) float64 {
	matches := stockfishNPSRE.FindStringSubmatch(log)
	if len(matches) != 2 {
		return 0
	}
	v, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0
	}
	return v
}

func medianFloat64(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	sort.Float64s(v)
	mid := len(v) / 2
	if len(v)%2 == 0 {
		return (v[mid-1] + v[mid]) / 2
	}
	return v[mid]
}

func (b Benchmark) runOpenSSL(ctx context.Context, req RunRequest) (RunOutput, bool, error) {
	asset, ok := assets.FindWorker("openssl-speed", runtime.GOOS, runtime.GOARCH)
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
	// Run fixed algorithm subset
	algorithms := []string{"sha256"}
	args := []string{"speed", "-elapsed", "-seconds", fmt.Sprintf("%d", opensslSeconds(req.Profile))}
	args = append(args, algorithms...)
	if threads > 1 {
		args = []string{"speed", "-elapsed", "-seconds", fmt.Sprintf("%d", opensslSeconds(req.Profile)), "-multi", fmt.Sprintf("%d", threads)}
		args = append(args, algorithms...)
	}

	proc, err := runner.RunProcess(ctx, worker.Path, args...)
	if err != nil {
		return RunOutput{
			StdoutTail: runner.Tail(proc.Stdout, 4096),
			StderrTail: runner.Tail(proc.Stderr, 4096),
		}, true, err
	}

	value := parseOpenSSLOps(proc.Stdout + "\n" + proc.Stderr)
	if value <= 0 {
		return RunOutput{
			StdoutTail: runner.Tail(proc.Stdout, 4096),
			StderrTail: runner.Tail(proc.Stderr, 4096),
		}, true, fmt.Errorf("openssl-speed: could not parse operations")
	}

	return RunOutput{
		Value:      value,
		Duration:   proc.Duration,
		Iterations: 0,
		StdoutTail: runner.Tail(proc.Stdout, 256),
		StderrTail: runner.Tail(proc.Stderr, 512),
		Note:       "openssl-speed embedded worker",
	}, true, nil
}

func opensslSeconds(profile string) int {
	switch profile {
	case "smoke":
		return 1
	case "quick":
		return 2
	case "extended":
		return 8
	default:
		return 4
	}
}

var opensslKRE = regexp.MustCompile(`([0-9.]+)k`)

func parseOpenSSLOps(log string) float64 {
	// OpenSSL speed output has lines like:
	// sha256   54321.23k   156789.01k   ...
	// Sum the "16 bytes" column (first numeric value) for each algorithm
	var total float64
	lines := strings.Split(log, "\n")
	for _, line := range lines {
		if strings.Contains(line, "k") && !strings.Contains(line, "type") {
			matches := opensslKRE.FindAllStringSubmatch(line, -1)
			if len(matches) > 0 {
				if v, err := strconv.ParseFloat(matches[0][1], 64); err == nil {
					total += v
				}
			}
		}
	}
	return total
}

func (b Benchmark) runZstd(ctx context.Context, req RunRequest) (RunOutput, bool, error) {
	asset, ok := assets.FindWorker("zstd", runtime.GOOS, runtime.GOARCH)
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

	corpusPath, corpusBytes, err := writeZstdCorpus(filepath.Dir(worker.Path), req.Profile)
	if err != nil {
		return RunOutput{}, true, err
	}
	proc, err := runner.RunProcess(ctx, worker.Path, "-b1", "-i1", "-T"+fmt.Sprintf("%d", threads), corpusPath)
	if err != nil {
		return RunOutput{
			StdoutTail: runner.Tail(proc.Stdout, 4096),
			StderrTail: runner.Tail(proc.Stderr, 4096),
		}, true, err
	}

	value := parseZstdMBs(proc.Stdout + "\n" + proc.Stderr)
	if value <= 0 {
		return RunOutput{
			StdoutTail: runner.Tail(proc.Stdout, 4096),
			StderrTail: runner.Tail(proc.Stderr, 4096),
		}, true, fmt.Errorf("zstd: could not parse MB/s")
	}

	return RunOutput{
		Value:      value,
		Duration:   proc.Duration,
		Iterations: uint64(corpusBytes),
		StdoutTail: runner.Tail(proc.Stdout, 256),
		StderrTail: runner.Tail(proc.Stderr, 512),
		Note:       "zstd embedded worker",
	}, true, nil
}

func writeZstdCorpus(dir, profile string) (string, int, error) {
	size := zstdCorpusBytes(profile)
	path := filepath.Join(dir, "zstd-corpus.bin")
	data := make([]byte, size)
	var x uint64 = 0x6a09e667f3bcc909
	for i := range data {
		x ^= x << 13
		x ^= x >> 7
		x ^= x << 17
		data[i] = byte(x) ^ byte(i/251)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", 0, err
	}
	return path, size, nil
}

func zstdCorpusBytes(profile string) int {
	switch profile {
	case "smoke":
		return 1 << 20
	case "quick":
		return 8 << 20
	case "extended":
		return 64 << 20
	default:
		return 32 << 20
	}
}

var zstdMBsRE = regexp.MustCompile(`([0-9.]+)\s+MB/s`)

func parseZstdMBs(log string) float64 {
	matches := zstdMBsRE.FindAllStringSubmatch(log, -1)
	if len(matches) == 0 {
		return 0
	}
	var product float64 = 1
	var count int
	for _, match := range matches {
		v, err := strconv.ParseFloat(match[1], 64)
		if err != nil || v <= 0 {
			continue
		}
		product *= v
		count++
	}
	if count == 0 {
		return 0
	}
	return math.Pow(product, 1/float64(count))
}

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

	proc, err := runner.RunProcess(ctx, worker.Path, "--threads", fmt.Sprintf("%d", threads))
	if err != nil {
		return RunOutput{
			StdoutTail: runner.Tail(proc.Stdout, 4096),
			StderrTail: runner.Tail(proc.Stderr, 4096),
		}, true, err
	}

	value := parseGGMLResult(proc.Stdout + "\n" + proc.Stderr)
	if value <= 0 {
		return RunOutput{
			StdoutTail: runner.Tail(proc.Stdout, 4096),
			StderrTail: runner.Tail(proc.Stderr, 4096),
		}, true, fmt.Errorf("ggml-ml-kernel: could not parse result")
	}

	return RunOutput{
		Value:      value,
		Duration:   proc.Duration,
		Iterations: 0,
		StdoutTail: runner.Tail(proc.Stdout, 256),
		StderrTail: runner.Tail(proc.Stderr, 512),
		Note:       "ggml-ml-kernel embedded worker",
	}, true, nil
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

func (b Benchmark) runTinyCC(ctx context.Context, req RunRequest) (RunOutput, bool, error) {
	asset, ok := assets.FindWorker("tinycc-compile", runtime.GOOS, runtime.GOARCH)
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

	nFiles := tinyCCFiles(req.Profile)
	workDir := filepath.Dir(worker.Path)
	sources, err := writeTinyCCCorpus(workDir, nFiles)
	if err != nil {
		return RunOutput{}, true, err
	}

	start := time.Now()
	var stdoutAll, stderrAll string
	var compiled uint64
	for i, source := range sources {
		obj := filepath.Join(workDir, fmt.Sprintf("tinycc-%04d.o", i))
		proc, err := runner.RunProcess(ctx, worker.Path, "-c", "-o", obj, source)
		stdoutAll += proc.Stdout
		stderrAll += proc.Stderr
		if err != nil {
			return RunOutput{
				Duration:   time.Since(start),
				StdoutTail: runner.Tail(stdoutAll, 4096),
				StderrTail: runner.Tail(stderrAll, 4096),
			}, true, err
		}
		compiled++
	}
	elapsed := time.Since(start)
	if compiled == 0 || elapsed <= 0 {
		return RunOutput{
			Duration:   elapsed,
			StdoutTail: runner.Tail(stdoutAll, 4096),
			StderrTail: runner.Tail(stderrAll, 4096),
		}, true, fmt.Errorf("tinycc-compile: no files compiled")
	}

	value := float64(compiled) / elapsed.Seconds()
	return RunOutput{
		Value:      value,
		Duration:   elapsed,
		Iterations: compiled,
		StdoutTail: runner.Tail(stdoutAll, 256),
		StderrTail: runner.Tail(stderrAll, 512),
		Note:       "tinycc-compile embedded worker",
	}, true, nil
}

func tinyCCFiles(profile string) int {
	switch profile {
	case "smoke":
		return 10
	case "quick":
		return 30
	case "extended":
		return 200
	default:
		return 50
	}
}

func writeTinyCCCorpus(dir string, nFiles int) ([]string, error) {
	templates := []string{
		"int fib_%[1]d(int n){if(n<=1)return n;return fib_%[1]d(n-1)+fib_%[1]d(n-2);} int f_%[1]d(void){return fib_%[1]d(%[2]d);}\n",
		"int gcd_%[1]d(int a,int b){while(b){int t=b;b=a%%b;a=t;}return a;} int f_%[1]d(void){return gcd_%[1]d(%[2]d,%[3]d);}\n",
		"int prime_%[1]d(int n){if(n<2)return 0;for(int i=2;i*i<=n;i++)if(n%%i==0)return 0;return 1;} int f_%[1]d(void){return prime_%[1]d(%[2]d);}\n",
		"void sort_%[1]d(int*a,int n){for(int i=0;i<n-1;i++)for(int j=i+1;j<n;j++)if(a[i]>a[j]){int t=a[i];a[i]=a[j];a[j]=t;}} int f_%[1]d(void){int a[4]={%[2]d,%[3]d,7,3};sort_%[1]d(a,4);return a[0];}\n",
	}
	paths := make([]string, 0, nFiles)
	for i := 0; i < nFiles; i++ {
		path := filepath.Join(dir, fmt.Sprintf("tinycc-%04d.c", i))
		body := fmt.Sprintf(templates[i%len(templates)], i, 11+i%17, 29+i%31)
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, nil
}

// Runtime budgets per profile (target wall-clock per test module per mode).
// Standard profile targets ~15 min on high-end CPU, ~30 min on low-end VPS.
// Smoke profile must complete in under 1 minute total for CI.
var profileBudgets = map[string]time.Duration{
	"smoke":    100 * time.Millisecond,
	"quick":    400 * time.Millisecond,
	"standard": 800 * time.Millisecond,
	"extended": 2 * time.Second,
}

func profileDuration(profile string) time.Duration {
	if d, ok := profileBudgets[profile]; ok {
		return d
	}
	return profileBudgets["standard"]
}

func DefaultBenchmarks() []Benchmark {
	return []Benchmark{
		def("sysbench", "events_per_second", "eps", "https://github.com/akopytov/sysbench", "GPL-2.0-or-later", 1.0),
		def("c-ray", "rays_per_second", "rps", "https://github.com/vkoskiv/c-ray", "MIT", 1.4),
		def("stockfish", "nodes_per_second", "nps", "https://github.com/official-stockfish/Stockfish", "GPL-3.0", 1.7),
		def("sqlite-speedtest", "tests_per_second", "tps", "https://www.sqlite.org/src/", "Public Domain", 1.2),
		def("openssl-speed", "operations_per_second", "ops", "https://github.com/openssl/openssl", "Apache-2.0", 1.5),
		def("zstd", "megabytes_per_second", "MB/s", "https://github.com/facebook/zstd", "BSD-style", 1.3),
		def("ggml-ml-kernel", "operations_per_second", "ops", "https://github.com/ggml-org/llama.cpp", "MIT", 1.8),
		def("tinycc-compile", "files_per_second", "files/s", "https://github.com/TinyCC/tinycc", "LGPL-2.1", 1.1),
	}
}

func def(name, metric, unit, source, license string, weight float64) Benchmark {
	return Benchmark{
		Name:       name,
		MetricName: metric,
		Unit:       unit,
		Direction:  HigherIsBetter,
		Source:     source,
		Revision:   "pending-third-party-lock",
		License:    license,
		WorkerHash: "missing",
		Compiler:   "missing",
		BuildFlags: "missing embedded worker",
		Command:    "missing embedded worker",
		Seed:       hashName(name),
		Weight:     weight,
	}
}

func hashName(name string) uint64 {
	sum := sha256.Sum256([]byte(name))
	var out uint64
	for _, b := range sum[:8] {
		out = out<<8 | uint64(b)
	}
	return out
}

func Select(all []Benchmark, allow, skip map[string]bool) []Benchmark {
	out := make([]Benchmark, 0, len(all))
	for _, b := range all {
		if len(allow) > 0 && !allow[b.Name] {
			continue
		}
		if skip[b.Name] {
			continue
		}
		out = append(out, b)
	}
	return out
}

func Provenance(benches []Benchmark) []results.BenchmarkProvenance {
	out := make([]results.BenchmarkProvenance, 0, len(benches))
	for _, b := range benches {
		if asset, ok := assets.FindWorker(b.Name, runtime.GOOS, runtime.GOARCH); ok {
			out = append(out, results.BenchmarkProvenance{
				Name:         b.Name,
				Source:       asset.Source,
				Revision:     asset.Revision,
				License:      asset.License,
				WorkerSHA256: asset.SHA256,
				Compiler:     asset.Compiler,
				BuildFlags:   asset.BuildFlags,
				Command:      asset.Command,
			})
			continue
		}
		out = append(out, results.BenchmarkProvenance{
			Name:         b.Name,
			Source:       b.Source,
			Revision:     b.Revision,
			License:      b.License,
			WorkerSHA256: b.WorkerHash,
			Compiler:     b.Compiler,
			BuildFlags:   b.BuildFlags,
			Command:      b.Command,
		})
	}
	return out
}
