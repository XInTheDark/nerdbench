package results

import "testing"

func TestAggregateScores(t *testing.T) {
	got := AggregateScores([]BenchmarkResult{
		{Name: "a", Mode: "single", Status: "ok", Score: 1000},
		{Name: "b", Mode: "single", Status: "ok", Score: 4000},
		{Name: "a", Mode: "multi", Status: "ok", Score: 9000},
		{Name: "b", Mode: "multi", Status: "failed", Score: 999999},
	})
	if got.Single < 1999 || got.Single > 2001 {
		t.Fatalf("single score = %v, want geometric mean near 2000", got.Single)
	}
	if got.Multi < 8999 || got.Multi > 9001 {
		t.Fatalf("multi score = %v, want 9000", got.Multi)
	}
}

func TestScoreMetric(t *testing.T) {
	b := Baseline{
		Metrics: map[string]float64{
			"bench/ops/single": 50,
		},
	}
	got := ScoreMetric("bench", "single", "ops", 100, nil, b)
	if got != 2000 {
		t.Fatalf("score = %v, want 2000", got)
	}
}

func TestLoadBaselineBytes(t *testing.T) {
	data := []byte(`{
		"score_version": "test-version",
		"metrics": [
			{
				"benchmark": "sqlite-speedtest",
				"mode": "single",
				"metric": "tests_per_second",
				"baseline_value": 12.5
			},
			{
				"benchmark": "sqlite-speedtest",
				"mode": "multi",
				"metric": "tests_per_second",
				"baseline_value": 50
			}
		]
	}`)

	b, err := LoadBaselineBytes("test-version.json", data)
	if err != nil {
		t.Fatalf("LoadBaselineBytes failed: %v", err)
	}
	if b.ScoreVersion != "test-version" {
		t.Fatalf("ScoreVersion = %q, want test-version", b.ScoreVersion)
	}
	if b.BaselineHash == "" || b.BaselineHash == "internal-dev-baseline" {
		t.Fatalf("BaselineHash = %q, want computed hash", b.BaselineHash)
	}
	if got := b.Metrics["sqlite-speedtest/tests_per_second/single"]; got != 12.5 {
		t.Fatalf("single baseline = %v, want 12.5", got)
	}
	if got := ScoreMetric("sqlite-speedtest", "multi", "tests_per_second", 100, nil, b); got != 2000 {
		t.Fatalf("multi score = %v, want 2000", got)
	}
}

func TestLoadBaselineBytesRejectsEmptyUsableMetrics(t *testing.T) {
	data := []byte(`{"score_version":"bad","metrics":[{"benchmark":"x","mode":"single","metric":"ops","baseline_value":0}]}`)
	if _, err := LoadBaselineBytes("bad.json", data); err == nil {
		t.Fatal("LoadBaselineBytes should reject a baseline without usable positive metric values")
	}
}

func TestBaselineChangesScoreCalculation(t *testing.T) {
	// Two different baselines for the same benchmark should produce different scores
	baselineA := Baseline{
		Metrics: map[string]float64{
			"bench/ops/single": 100,
		},
	}
	baselineB := Baseline{
		Metrics: map[string]float64{
			"bench/ops/single": 200,
		},
	}
	measuredValue := 150.0

	scoreA := ScoreMetric("bench", "single", "ops", measuredValue, nil, baselineA)
	scoreB := ScoreMetric("bench", "single", "ops", measuredValue, nil, baselineB)

	if scoreA == scoreB {
		t.Fatalf("different baselines produced same score: %v == %v", scoreA, scoreB)
	}
	// With baseline 100: score = 1000 * 150/100 = 1500
	// With baseline 200: score = 1000 * 150/200 = 750
	if scoreA != 1500 {
		t.Fatalf("scoreA = %v, want 1500", scoreA)
	}
	if scoreB != 750 {
		t.Fatalf("scoreB = %v, want 750", scoreB)
	}
}
