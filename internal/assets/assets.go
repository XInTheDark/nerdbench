package assets

import "os"

type WorkerAsset struct {
	Name       string
	Benchmark  string
	OS         string
	Arch       string
	SHA256     string
	Bytes      []byte
	Source     string
	Revision   string
	License    string
	Compiler   string
	BuildFlags string
	Command    string
}

var generatedWorkers []WorkerAsset

func Workers() []WorkerAsset {
	out := make([]WorkerAsset, len(generatedWorkers))
	copy(out, generatedWorkers)
	return out
}

func FindWorker(benchmark, osName, arch string) (WorkerAsset, bool) {
	if os.Getenv("NERDBENCH_DISABLE_WORKERS") == "1" {
		return WorkerAsset{}, false
	}
	for _, worker := range Workers() {
		if worker.Benchmark == benchmark && worker.OS == osName && worker.Arch == arch {
			return worker, true
		}
	}
	return WorkerAsset{}, false
}
