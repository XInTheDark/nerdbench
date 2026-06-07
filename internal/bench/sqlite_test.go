package bench

import "testing"

func TestParseSQLiteTotalSeconds(t *testing.T) {
	log := " 990 - ANALYZE.....................................    0.000s\n       TOTAL.......................................................    0.015s\n"
	got := parseSQLiteTotalSeconds(log)
	if got != 0.015 {
		t.Fatalf("parseSQLiteTotalSeconds() = %v, want 0.015", got)
	}
}
