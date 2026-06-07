package bench

import "testing"

func TestParseOpenSSLOps(t *testing.T) {
	tests := []struct {
		name string
		log  string
		want float64
	}{
		{
			name: "standard_output",
			log:  "type             16 bytes     64 bytes    256 bytes   1024 bytes   8192 bytes\nsha256           54321.23k   156789.01k   345678.90k   567890.12k   678901.23k\naes-256-gcm     123456.78k   345678.90k   567890.12k   789012.34k   890123.45k",
			want: 54321.23 + 123456.78,
		},
		{
			name: "missing",
			log:  "no benchmark data",
			want: 0,
		},
		{
			name: "single_algorithm",
			log:  "sha256   98765.43k   234567.89k   345678.90k",
			want: 98765.43,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseOpenSSLOps(tt.log)
			if got != tt.want {
				t.Fatalf("parseOpenSSLOps() = %v, want %v", got, tt.want)
			}
		})
	}
}
