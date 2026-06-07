package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/nerdbench/nerdbench/internal/bench"
	"github.com/nerdbench/nerdbench/internal/progress"
	"github.com/nerdbench/nerdbench/internal/results"
	"github.com/nerdbench/nerdbench/internal/system"
)

var version = "0.1.0-dev"

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) > 0 && args[0] == "run" {
		args = args[1:]
	}
	cfg, err := parseRunConfig(args)
	if err != nil {
		return err
	}
	if cfg.ShowVersion {
		fmt.Fprintln(stdout, version)
		return nil
	}
	return runBenchmarks(cfg, stdout, stderr)
}

type runConfig struct {
	Profile     string
	Threads     int
	SingleOnly  bool
	MultiOnly   bool
	Bench       string
	Skip        string
	Format      string
	Progress    string
	Output      string
	Timeout     time.Duration
	TmpDir      string
	NoCleanup   bool
	ShowVersion bool
}

func parseRunConfig(args []string) (runConfig, error) {
	cfg := runConfig{
		Profile:  "standard",
		Threads:  runtime.NumCPU(),
		Format:   "text",
		Progress: "text",
		Timeout:  30 * time.Minute,
	}

	fs := flag.NewFlagSet("nerdbench run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&cfg.Profile, "profile", cfg.Profile, "profile: smoke, quick, standard, extended")
	fs.IntVar(&cfg.Threads, "threads", cfg.Threads, "thread count")
	fs.BoolVar(&cfg.SingleOnly, "single-only", false, "run only single-core tests")
	fs.BoolVar(&cfg.MultiOnly, "multi-only", false, "run only multi-core tests")
	fs.StringVar(&cfg.Bench, "bench", "", "comma-separated benchmark allowlist")
	fs.StringVar(&cfg.Skip, "skip", "", "comma-separated benchmark skip list")
	fs.StringVar(&cfg.Format, "format", cfg.Format, "format: text or json")
	fs.StringVar(&cfg.Progress, "progress", cfg.Progress, "progress: text, ndjson, or none")
	fs.StringVar(&cfg.Output, "o", "", "write full JSON result")
	fs.StringVar(&cfg.Output, "output", "", "write full JSON result")
	fs.DurationVar(&cfg.Timeout, "timeout", cfg.Timeout, "global timeout")
	fs.StringVar(&cfg.TmpDir, "tmpdir", "", "override extraction directory")
	fs.BoolVar(&cfg.NoCleanup, "no-cleanup", false, "keep extracted workers for debugging")
	fs.BoolVar(&cfg.ShowVersion, "version", false, "print version")
	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	if cfg.Threads < 1 {
		return cfg, errors.New("--threads must be at least 1")
	}
	if cfg.SingleOnly && cfg.MultiOnly {
		return cfg, errors.New("--single-only and --multi-only cannot both be set")
	}
	if !validProfile(cfg.Profile) {
		return cfg, fmt.Errorf("unsupported profile %q", cfg.Profile)
	}
	if cfg.Format != "text" && cfg.Format != "json" {
		return cfg, fmt.Errorf("unsupported format %q", cfg.Format)
	}
	if cfg.Progress != "text" && cfg.Progress != "ndjson" && cfg.Progress != "none" {
		return cfg, fmt.Errorf("unsupported progress %q", cfg.Progress)
	}
	return cfg, nil
}

func validProfile(profile string) bool {
	switch profile {
	case "smoke", "quick", "standard", "extended":
		return true
	default:
		return false
	}
}

func runBenchmarks(cfg runConfig, stdout, stderr io.Writer) error {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	progressSink := progress.New(cfg.Progress, stderr)
	started := time.Now().UTC()
	sys := system.Collect()
	baseline := results.LoadBaseline()
	selected := bench.Select(bench.DefaultBenchmarks(), splitSet(cfg.Bench), splitSet(cfg.Skip))
	modes := []bench.Mode{bench.ModeSingle, bench.ModeMulti}
	if cfg.SingleOnly {
		modes = []bench.Mode{bench.ModeSingle}
	}
	if cfg.MultiOnly {
		modes = []bench.Mode{bench.ModeMulti}
	}

	total := len(selected) * len(modes)
	var benchResults []results.BenchmarkResult
	runErrors := []results.RunError{}
	index := 0
	for _, b := range selected {
		for _, mode := range modes {
			index++
			threads := 1
			if mode == bench.ModeMulti {
				threads = cfg.Threads
			}
			progressSink.Start(index, total, b.Name, string(mode), threads)
			out, err := b.Run(ctx, bench.RunRequest{
				Profile: cfg.Profile,
				Mode:    mode,
				Threads: threads,
			})
			if err != nil {
				progressSink.Error(index, total, b.Name, string(mode), err)
				runErrors = append(runErrors, results.RunError{
					Benchmark: b.Name,
					Mode:      string(mode),
					Message:   err.Error(),
				})
				benchResults = append(benchResults, results.BenchmarkResult{
					Name:       b.Name,
					Mode:       string(mode),
					Status:     "failed",
					Threads:    threads,
					DurationMS: out.Duration.Milliseconds(),
					Metric:     results.Metric{Name: b.MetricName, Unit: b.Unit},
					Log:        results.LogTail{StdoutTail: out.StdoutTail, StderrTail: out.StderrTail},
				})
				continue
			}
			score := results.ScoreMetric(b.Name, string(mode), b.MetricName, out.Value, b.Direction, baseline)
			progressSink.Done(index, total, b.Name, string(mode), out.Value, b.Unit, score)
			benchResults = append(benchResults, results.BenchmarkResult{
				Name:       b.Name,
				Mode:       string(mode),
				Status:     "ok",
				Threads:    threads,
				DurationMS: out.Duration.Milliseconds(),
				Metric:     results.Metric{Name: b.MetricName, Value: out.Value, Unit: b.Unit},
				Score:      score,
				Raw: map[string]any{
					"iterations": out.Iterations,
					"note":       out.Note,
				},
				Log: results.LogTail{StdoutTail: out.StdoutTail, StderrTail: out.StderrTail},
			})
		}
	}

	status := "ok"
	if len(runErrors) > 0 {
		status = "failed"
	}
	scoreSummary := results.AggregateScores(benchResults)
	doc := results.Result{
		SchemaVersion:    1,
		NerdBenchVersion: version,
		ScoreVersion:     baseline.ScoreVersion,
		Profile:          cfg.Profile,
		StartedAt:        started.Format(time.RFC3339),
		DurationMS:       time.Since(started).Milliseconds(),
		Status:           status,
		System:           sys,
		Provenance: results.Provenance{
			ManifestHash: baseline.ManifestHash,
			BaselineHash: baseline.BaselineHash,
			Benchmarks:   bench.Provenance(selected),
		},
		Scores:     scoreSummary,
		Benchmarks: benchResults,
		Errors:     runErrors,
	}

	if cfg.Output != "" {
		if err := results.WriteJSONFile(cfg.Output, doc); err != nil {
			return err
		}
	}
	if cfg.Format == "json" {
		if err := results.WriteJSON(stdout, doc); err != nil {
			return err
		}
		if status != "ok" {
			return errors.New("one or more benchmarks failed")
		}
		return nil
	}
	printText(stdout, doc)
	if status != "ok" {
		return errors.New("one or more benchmarks failed")
	}
	return nil
}

func splitSet(value string) map[string]bool {
	out := map[string]bool{}
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out[part] = true
		}
	}
	return out
}

func printText(w io.Writer, doc results.Result) {
	fmt.Fprintf(w, "NerdBench %s (%s)\n", doc.NerdBenchVersion, doc.Profile)
	fmt.Fprintf(w, "Single Core: %.0f\n", doc.Scores.Single)
	fmt.Fprintf(w, "Multi Core:  %.0f\n", doc.Scores.Multi)
	fmt.Fprintln(w)
	for _, b := range doc.Benchmarks {
		if b.Status != "ok" {
			fmt.Fprintf(w, "%-22s %-6s failed\n", b.Name, b.Mode)
			continue
		}
		fmt.Fprintf(w, "%-22s %-6s %12.2f %-8s score %.0f\n", b.Name, b.Mode, b.Metric.Value, b.Metric.Unit, b.Score)
	}
}
