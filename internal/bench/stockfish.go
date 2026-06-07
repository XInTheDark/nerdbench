package bench

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"time"

	"github.com/nerdbench/nerdbench/internal/assets"
	"github.com/nerdbench/nerdbench/internal/runner"
)

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
	var allNPS []float64
	var stdoutAll, stderrAll string
	var elapsed time.Duration
	runtimeSeconds := stockfishRuntimeSeconds(req.Profile)

	for i := 0; i < stockfishRuns(req.Profile); i++ {
		proc, err := runner.RunProcess(
			ctx,
			worker.Path,
			"speedtest",
			strconv.Itoa(threads),
			"128",
			strconv.Itoa(runtimeSeconds),
		)
		stdoutAll += proc.Stdout
		stderrAll += proc.Stderr
		elapsed += proc.Duration
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
		Duration:   elapsed,
		Iterations: 0,
		StdoutTail: runner.Tail(stdoutAll, 256),
		StderrTail: runner.Tail(stderrAll, 512),
		Note:       "stockfish embedded worker",
	}, true, nil
}

func stockfishRuns(profile string) int {
	return 1
}

func stockfishRuntimeSeconds(profile string) int {
	return maxInt(1, int(math.Round(profileDuration(profile).Seconds())))
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
