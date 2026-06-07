package bench

import (
	"crypto/sha256"
	"runtime"

	"github.com/nerdbench/nerdbench/internal/assets"
	"github.com/nerdbench/nerdbench/internal/results"
)

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
