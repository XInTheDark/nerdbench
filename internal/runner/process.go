package runner

import (
	"bytes"
	"context"
	"os/exec"
	"time"
)

type ProcessResult struct {
	Stdout   string
	Stderr   string
	Duration time.Duration
}

func RunProcess(ctx context.Context, path string, args ...string) (ProcessResult, error) {
	return RunProcessInDir(ctx, "", path, args...)
}

func RunProcessInDir(ctx context.Context, dir, path string, args ...string) (ProcessResult, error) {
	start := time.Now()
	cmd := exec.CommandContext(ctx, path, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return ProcessResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: time.Since(start),
	}, err
}

func Tail(s string, maxBytes int) string {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s
	}
	return s[len(s)-maxBytes:]
}
