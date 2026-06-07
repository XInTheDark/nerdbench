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
	return maxInt(1, int(math.Round(profileDuration(profile).Seconds())))
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
