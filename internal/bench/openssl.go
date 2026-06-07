package bench

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/nerdbench/nerdbench/internal/assets"
	"github.com/nerdbench/nerdbench/internal/runner"
)

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
	blockSizes := 6
	return maxInt(1, int(math.Ceil(profileDuration(profile).Seconds()/float64(blockSizes))))
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
