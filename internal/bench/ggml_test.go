package bench

import "testing"

func TestParseGGMLResult(t *testing.T) {
	tests := []struct {
		name string
		log  string
		want float64
	}{
		{
			name: "standard",
			log:  "NERDBENCH_GGML_RESULT:12.345678\nggml 512x512x512 matmul x10 iter: 12.346 GFLOP/s\n",
			want: 12.345678,
		},
		{
			name: "missing",
			log:  "no benchmark data",
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseGGMLResult(tt.log)
			if got != tt.want {
				t.Fatalf("parseGGMLResult() = %v, want %v", got, tt.want)
			}
		})
	}
}
