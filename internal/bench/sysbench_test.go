package bench

import "testing"

func TestParseSysbenchEPS(t *testing.T) {
	tests := []struct {
		name string
		log  string
		want float64
	}{
		{
			name: "standard",
			log:  "CPU speed:\n    events per second:  1234.56\n\nGeneral statistics:",
			want: 1234.56,
		},
		{
			name: "missing",
			log:  "no relevant data here",
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseSysbenchEPS(tt.log)
			if got != tt.want {
				t.Fatalf("parseSysbenchEPS() = %v, want %v", got, tt.want)
			}
		})
	}
}
