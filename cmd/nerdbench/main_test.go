package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestRunSmokeJSONContract(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := run([]string{"run", "--profile", "smoke", "--format", "json", "--progress", "none", "--bench", "sysbench"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run failed: %v\nstderr: %s", err, stderr.String())
	}

	var doc map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, stdout.String())
	}
	if doc["schema_version"].(float64) != 1 {
		t.Fatalf("schema_version = %v", doc["schema_version"])
	}
	scores := doc["scores"].(map[string]any)
	if _, ok := scores["single"]; !ok {
		t.Fatal("scores.single missing")
	}
	if _, ok := scores["multi"]; !ok {
		t.Fatal("scores.multi missing")
	}
	if got := doc["errors"]; got == nil {
		t.Fatal("errors should encode as [] not null")
	}
}

func TestRunJSONFailureReturnsError(t *testing.T) {
	t.Setenv("NERDBENCH_DISABLE_WORKERS", "1")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := run([]string{"run", "--profile", "smoke", "--format", "json", "--progress", "none", "--bench", "sysbench"}, &stdout, &stderr)
	if err == nil {
		t.Fatalf("run succeeded; want failure\nstdout: %s\nstderr: %s", stdout.String(), stderr.String())
	}

	var doc map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if status := doc["status"].(string); status != "failed" {
		t.Fatalf("status = %q, want failed", status)
	}
	benchmarks, ok := doc["benchmarks"].([]any)
	if !ok {
		t.Fatalf("benchmarks field missing or not an array")
	}
	if len(benchmarks) == 0 {
		t.Fatal("benchmarks should contain the failed test module")
	}
	for _, b := range benchmarks {
		bm, ok := b.(map[string]any)
		if !ok {
			t.Fatalf("benchmark entry is not an object: %v", b)
		}
		if bmStatus := bm["status"].(string); bmStatus != "failed" {
			t.Fatalf("benchmark status = %q, want failed", bmStatus)
		}
		if score, ok := bm["score"].(float64); ok && score > 0 {
			t.Fatalf("failed benchmark has positive score: %v", score)
		}
	}
}

func TestRunTextFailureReturnsError(t *testing.T) {
	t.Setenv("NERDBENCH_DISABLE_WORKERS", "1")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := run([]string{"run", "--profile", "smoke", "--format", "text", "--progress", "none", "--bench", "sysbench"}, &stdout, &stderr)
	if err == nil {
		t.Fatalf("run succeeded; want failure\nstdout: %s\nstderr: %s", stdout.String(), stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("failed")) {
		t.Fatalf("text output does not report failure: %s", stdout.String())
	}
}
