package bench

import (
	"os"
	"testing"
)

func TestWriteTinyCCCorpus(t *testing.T) {
	paths, err := writeTinyCCCorpus(t.TempDir(), 6)
	if err != nil {
		t.Fatalf("writeTinyCCCorpus failed: %v", err)
	}
	if len(paths) != 6 {
		t.Fatalf("len(paths) = %d, want 6", len(paths))
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("corpus file missing: %v", err)
		}
		if len(data) == 0 {
			t.Fatalf("corpus file is empty: %s", path)
		}
	}
}
