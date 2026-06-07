package bench

import "testing"

func TestParseZstdMBs(t *testing.T) {
	tests := []struct {
		name string
		log  string
		want float64
	}{
		{
			name: "standard_output",
			log:  " *** Benchmark ** : Compression Level, 1 thread\n     1#_1         :   1234.5 MB/s   4567.8 MB/s",
			want: 2374.6471527365916,
		},
		{
			name: "missing",
			log:  "no benchmark data",
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseZstdMBs(tt.log)
			if got != tt.want {
				t.Fatalf("parseZstdMBs() = %v, want %v", got, tt.want)
			}
		})
	}
}
