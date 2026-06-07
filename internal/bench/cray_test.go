package bench

import "testing"

func TestParseCRaySeconds(t *testing.T) {
	tests := []struct {
		name string
		log  string
		want float64
	}{
		{name: "milliseconds", log: "INFO Finished render in 21ms", want: 0.021},
		{name: "seconds", log: "INFO Finished render in 17.52s", want: 17.52},
		{name: "missing", log: "INFO nope", want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCRaySeconds(tt.log)
			if got != tt.want {
				t.Fatalf("parseCRaySeconds() = %v, want %v", got, tt.want)
			}
		})
	}
}
