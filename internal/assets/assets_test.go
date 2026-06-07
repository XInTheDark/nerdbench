package assets

import "testing"

func TestFindWorkerCanBeDisabled(t *testing.T) {
	t.Setenv("NERDBENCH_DISABLE_WORKERS", "1")
	if _, ok := FindWorker("sysbench", "linux", "amd64"); ok {
		t.Fatal("disabled workers should not be found")
	}
}
