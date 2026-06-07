package bench

import (
	"context"
	"fmt"
	"runtime"
	"time"
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
