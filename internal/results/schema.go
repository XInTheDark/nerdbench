package results

import (
	"encoding/json"
	"io"
	"os"
)

type Result struct {
	SchemaVersion    int               `json:"schema_version"`
	NerdBenchVersion string            `json:"nerdbench_version"`
	ScoreVersion     string            `json:"score_version"`
	Profile          string            `json:"profile"`
	StartedAt        string            `json:"started_at"`
	DurationMS       int64             `json:"duration_ms"`
	Status           string            `json:"status"`
	System           SystemInfo        `json:"system"`
	Provenance       Provenance        `json:"provenance"`
	Scores           ScoreSummary      `json:"scores"`
	Benchmarks       []BenchmarkResult `json:"benchmarks"`
	Errors           []RunError        `json:"errors"`
}

type SystemInfo struct {
	OS             string `json:"os"`
	Arch           string `json:"arch"`
	Kernel         string `json:"kernel"`
	CPUModel       string `json:"cpu_model"`
	LogicalCPUs    int    `json:"logical_cpus"`
	MemoryBytes    uint64 `json:"memory_bytes"`
	Virtualization string `json:"virtualization"`
}

type Provenance struct {
	ManifestHash string                `json:"manifest_hash"`
	BaselineHash string                `json:"baseline_hash"`
	Benchmarks   []BenchmarkProvenance `json:"benchmarks"`
}

type BenchmarkProvenance struct {
	Name         string `json:"name"`
	Source       string `json:"source"`
	Revision     string `json:"revision"`
	License      string `json:"license"`
	WorkerSHA256 string `json:"worker_sha256"`
	Compiler     string `json:"compiler"`
	BuildFlags   string `json:"build_flags"`
	Command      string `json:"command"`
}

type ScoreSummary struct {
	Single float64 `json:"single"`
	Multi  float64 `json:"multi"`
}

type BenchmarkResult struct {
	Name       string         `json:"name"`
	Mode       string         `json:"mode"`
	Status     string         `json:"status"`
	Threads    int            `json:"threads"`
	DurationMS int64          `json:"duration_ms"`
	Metric     Metric         `json:"metric"`
	Score      float64        `json:"score,omitempty"`
	Raw        map[string]any `json:"raw,omitempty"`
	Log        LogTail        `json:"log,omitempty"`
}

type Metric struct {
	Name  string  `json:"name"`
	Value float64 `json:"value,omitempty"`
	Unit  string  `json:"unit"`
}

type LogTail struct {
	StdoutTail string `json:"stdout_tail,omitempty"`
	StderrTail string `json:"stderr_tail,omitempty"`
}

type RunError struct {
	Benchmark string `json:"benchmark,omitempty"`
	Mode      string `json:"mode,omitempty"`
	Message   string `json:"message"`
}

func WriteJSON(w io.Writer, doc Result) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}

func WriteJSONFile(path string, doc Result) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return WriteJSON(f, doc)
}
