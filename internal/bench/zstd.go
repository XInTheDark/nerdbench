package bench

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"

	"github.com/nerdbench/nerdbench/internal/assets"
	"github.com/nerdbench/nerdbench/internal/runner"
)

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
	proc, err := runner.RunProcess(ctx, worker.Path, "-b1", "-i"+strconv.Itoa(zstdIterationSeconds(req.Profile)), "-T"+fmt.Sprintf("%d", threads), corpusPath)
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
		return 16 << 20
	case "extended":
		return 128 << 20
	default:
		return 64 << 20
	}
}

func zstdIterationSeconds(profile string) int {
	return maxInt(1, int(math.Ceil(profileDuration(profile).Seconds()/3)))
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
