package results

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"math"
	"path"
	"sort"
)

//go:embed baselines
var baselineFS embed.FS

type Baseline struct {
	ScoreVersion string
	ManifestHash string
	BaselineHash string
	Metrics      map[string]float64
}

func DefaultBaseline() Baseline {
	metrics := map[string]float64{}
	names := []string{
		"sysbench/events_per_second",
		"c-ray/rays_per_second",
		"stockfish/nodes_per_second",
		"sqlite-speedtest/tests_per_second",
		"openssl-speed/operations_per_second",
		"zstd/megabytes_per_second",
		"ggml-ml-kernel/operations_per_second",
		"tinycc-compile/files_per_second",
	}
	for _, name := range names {
		metrics[name+"/single"] = 10000
	}
	return Baseline{
		ScoreVersion: "2026-06-07-dev",
		ManifestHash: "internal-dev-manifest",
		BaselineHash: "internal-dev-baseline",
		Metrics:      metrics,
	}
}

func LoadBaseline() Baseline {
	baseline, err := loadLatestEmbeddedBaseline()
	if err != nil {
		return DefaultBaseline()
	}
	return baseline
}

func loadLatestEmbeddedBaseline() (Baseline, error) {
	entries, err := fs.ReadDir(baselineFS, "baselines")
	if err != nil {
		return Baseline{}, err
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() || path.Ext(entry.Name()) != ".json" {
			continue
		}
		names = append(names, entry.Name())
	}
	if len(names) == 0 {
		return Baseline{}, errors.New("no embedded baseline json files")
	}
	sort.Strings(names)
	return LoadBaselineBytes(names[len(names)-1], mustReadEmbeddedBaseline(names[len(names)-1]))
}

func mustReadEmbeddedBaseline(name string) []byte {
	data, err := baselineFS.ReadFile(path.Join("baselines", name))
	if err != nil {
		panic(err)
	}
	return data
}

type baselineFile struct {
	ScoreVersion string `json:"score_version"`
	Metrics      []struct {
		Benchmark     string  `json:"benchmark"`
		Mode          string  `json:"mode"`
		Metric        string  `json:"metric"`
		BaselineValue float64 `json:"baseline_value"`
	} `json:"metrics"`
}

func LoadBaselineBytes(name string, data []byte) (Baseline, error) {
	var doc baselineFile
	if err := json.Unmarshal(data, &doc); err != nil {
		return Baseline{}, err
	}
	if doc.ScoreVersion == "" {
		return Baseline{}, errors.New("baseline score_version is required")
	}
	metrics := map[string]float64{}
	for _, metric := range doc.Metrics {
		if metric.Benchmark == "" || metric.Mode == "" || metric.Metric == "" || metric.BaselineValue <= 0 {
			continue
		}
		if metric.Mode != "single" {
			continue
		}
		metrics[metric.Benchmark+"/"+metric.Metric+"/"+metric.Mode] = metric.BaselineValue
	}
	if len(metrics) == 0 {
		return Baseline{}, errors.New("baseline has no usable metrics")
	}
	sum := sha256.Sum256(data)
	return Baseline{
		ScoreVersion: doc.ScoreVersion,
		ManifestHash: "internal-dev-manifest",
		BaselineHash: "sha256:" + hex.EncodeToString(sum[:]),
		Metrics:      metrics,
	}, nil
}

func ScoreMetric(benchmark, mode, metric string, value float64, direction any, baseline Baseline) float64 {
	key := benchmark + "/" + metric + "/single"
	base := baseline.Metrics[key]
	if base <= 0 || value <= 0 {
		return 0
	}
	return 1000 * (value / base)
}

func AggregateScores(results []BenchmarkResult) ScoreSummary {
	var single []float64
	var multi []float64
	for _, r := range results {
		if r.Status != "ok" || r.Score <= 0 {
			continue
		}
		switch r.Mode {
		case "single":
			single = append(single, r.Score)
		case "multi":
			multi = append(multi, r.Score)
		}
	}
	return ScoreSummary{
		Single: geometricMean(single),
		Multi:  geometricMean(multi),
	}
}

func geometricMean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, value := range values {
		if value <= 0 {
			return 0
		}
		sum += math.Log(value)
	}
	return math.Exp(sum / float64(len(values)))
}
