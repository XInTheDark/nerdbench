package progress

import (
	"encoding/json"
	"fmt"
	"io"
)

type Sink struct {
	mode string
	w    io.Writer
}

func New(mode string, w io.Writer) Sink {
	return Sink{mode: mode, w: w}
}

func (s Sink) Start(index, total int, name, mode string, threads int) {
	if s.mode == "none" {
		return
	}
	if s.mode == "ndjson" {
		s.event(map[string]any{"event": "bench_started", "index": index, "total": total, "name": name, "mode": mode, "threads": threads})
		return
	}
	fmt.Fprintf(s.w, "[%d/%d] %s %s running... threads=%d\n", index, total, name, mode, threads)
}

func (s Sink) Done(index, total int, name, mode string, value float64, unit string, score float64) {
	if s.mode == "none" {
		return
	}
	if s.mode == "ndjson" {
		s.event(map[string]any{"event": "bench_finished", "index": index, "total": total, "name": name, "mode": mode, "value": value, "unit": unit, "score": score})
		return
	}
	fmt.Fprintf(s.w, "[%d/%d] %s %s done: %.2f %s, score %.0f\n", index, total, name, mode, value, unit, score)
}

func (s Sink) Error(index, total int, name, mode string, err error) {
	if s.mode == "none" {
		return
	}
	if s.mode == "ndjson" {
		s.event(map[string]any{"event": "bench_failed", "index": index, "total": total, "name": name, "mode": mode, "error": err.Error()})
		return
	}
	fmt.Fprintf(s.w, "[%d/%d] %s %s failed: %s\n", index, total, name, mode, err)
}

func (s Sink) event(event map[string]any) {
	_ = json.NewEncoder(s.w).Encode(event)
}
