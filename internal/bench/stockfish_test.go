package bench

import "testing"

func TestParseStockfishNPS(t *testing.T) {
	tests := []struct {
		name string
		log  string
		want float64
	}{
		{
			name: "standard",
			log:  "Nodes searched  : 25738921\nNodes/second    : 428981.33\n",
			want: 428981.33,
		},
		{
			name: "in_bench_output",
			log:  "info depth 20 score cp 15 nodes 123456 nps 234567\nNodes/second : 345678.90",
			want: 345678.90,
		},
		{
			name: "missing",
			log:  "no benchmark data",
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseStockfishNPS(tt.log)
			if got != tt.want {
				t.Fatalf("parseStockfishNPS() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMedianFloat64(t *testing.T) {
	tests := []struct {
		name string
		vals []float64
		want float64
	}{
		{name: "odd", vals: []float64{1, 3, 2}, want: 2},
		{name: "even", vals: []float64{1, 2, 3, 4}, want: 2.5},
		{name: "single", vals: []float64{42}, want: 42},
		{name: "empty", vals: []float64{}, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := medianFloat64(tt.vals)
			if got != tt.want {
				t.Fatalf("medianFloat64() = %v, want %v", got, tt.want)
			}
		})
	}
}
