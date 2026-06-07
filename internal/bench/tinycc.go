package bench

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nerdbench/nerdbench/internal/assets"
	"github.com/nerdbench/nerdbench/internal/runner"
)

func (b Benchmark) runTinyCC(ctx context.Context, req RunRequest) (RunOutput, bool, error) {
	asset, ok := assets.FindWorker("tinycc-compile", runtime.GOOS, runtime.GOARCH)
	if !ok {
		return RunOutput{}, false, nil
	}
	worker, err := runner.ExtractWorker(runner.ExtractRequest{
		Name:      asset.Name,
		Bytes:     asset.Bytes,
		SHA256Hex: asset.SHA256,
	})
	if err != nil {
		return RunOutput{}, true, err
	}
	defer worker.Cleanup()

	threads := req.Threads
	if req.Mode == ModeSingle {
		threads = 1
	}
	workDir := filepath.Dir(worker.Path)
	sources, err := writeTinyCCCorpusWithComplexity(workDir, tinyCCCorpusFiles(req.Profile), tinyCCFunctionsPerFile(req.Profile))
	if err != nil {
		return RunOutput{}, true, err
	}

	start := time.Now()
	deadline := start.Add(profileDuration(req.Profile))
	var stdoutAll, stderrAll string
	var nextID uint64
	var compiled uint64
	var stopped int32
	var firstErr error
	var mu sync.Mutex
	var wg sync.WaitGroup

	for workerID := 0; workerID < threads; workerID++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for time.Now().Before(deadline) && atomic.LoadInt32(&stopped) == 0 {
				id := atomic.AddUint64(&nextID, 1) - 1
				source := sources[int(id)%len(sources)]
				obj := filepath.Join(workDir, fmt.Sprintf("tinycc-%03d-%08d.o", workerID, id))
				proc, err := runner.RunProcess(ctx, worker.Path, "-c", "-o", obj, source)
				if proc.Stdout != "" || proc.Stderr != "" {
					mu.Lock()
					stdoutAll += proc.Stdout
					stderrAll += proc.Stderr
					mu.Unlock()
				}
				if err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					mu.Unlock()
					atomic.StoreInt32(&stopped, 1)
					return
				}
				atomic.AddUint64(&compiled, 1)
			}
		}(workerID)
	}
	wg.Wait()

	if firstErr != nil {
		return RunOutput{
			Duration:   time.Since(start),
			StdoutTail: runner.Tail(stdoutAll, 4096),
			StderrTail: runner.Tail(stderrAll, 4096),
		}, true, firstErr
	}
	elapsed := time.Since(start)
	if compiled == 0 || elapsed <= 0 {
		return RunOutput{
			Duration:   elapsed,
			StdoutTail: runner.Tail(stdoutAll, 4096),
			StderrTail: runner.Tail(stderrAll, 4096),
		}, true, fmt.Errorf("tinycc-compile: no files compiled")
	}

	value := float64(compiled) / elapsed.Seconds()
	return RunOutput{
		Value:      value,
		Duration:   elapsed,
		Iterations: compiled,
		StdoutTail: runner.Tail(stdoutAll, 256),
		StderrTail: runner.Tail(stderrAll, 512),
		Note:       "tinycc-compile embedded worker",
	}, true, nil
}

func tinyCCFiles(profile string) int {
	return tinyCCCorpusFiles(profile)
}

func tinyCCCorpusFiles(profile string) int {
	switch profile {
	case "smoke":
		return 16
	case "quick":
		return 64
	case "extended":
		return 512
	default:
		return 256
	}
}

func tinyCCFunctionsPerFile(profile string) int {
	switch profile {
	case "smoke":
		return 4
	case "quick":
		return 16
	case "extended":
		return 96
	default:
		return 64
	}
}

func writeTinyCCCorpus(dir string, nFiles int) ([]string, error) {
	return writeTinyCCCorpusWithComplexity(dir, nFiles, 1)
}

func writeTinyCCCorpusWithComplexity(dir string, nFiles, functionsPerFile int) ([]string, error) {
	templates := []string{
		"int fib_%[1]d(int n){if(n<=1)return n;return fib_%[1]d(n-1)+fib_%[1]d(n-2);} int f_%[1]d(void){return fib_%[1]d(%[2]d);}\n",
		"int gcd_%[1]d(int a,int b){while(b){int t=b;b=a%%b;a=t;}return a;} int f_%[1]d(void){return gcd_%[1]d(%[2]d,%[3]d);}\n",
		"int prime_%[1]d(int n){if(n<2)return 0;for(int i=2;i*i<=n;i++)if(n%%i==0)return 0;return 1;} int f_%[1]d(void){return prime_%[1]d(%[2]d);}\n",
		"void sort_%[1]d(int*a,int n){for(int i=0;i<n-1;i++)for(int j=i+1;j<n;j++)if(a[i]>a[j]){int t=a[i];a[i]=a[j];a[j]=t;}} int f_%[1]d(void){int a[4]={%[2]d,%[3]d,7,3};sort_%[1]d(a,4);return a[0];}\n",
	}
	paths := make([]string, 0, nFiles)
	for i := 0; i < nFiles; i++ {
		path := filepath.Join(dir, fmt.Sprintf("tinycc-%04d.c", i))
		var body strings.Builder
		for j := 0; j < functionsPerFile; j++ {
			id := i*functionsPerFile + j
			body.WriteString(fmt.Sprintf(templates[id%len(templates)], id, 11+id%17, 29+id%31))
		}
		body.WriteString(fmt.Sprintf("int tinycc_entry_%d(void){int s=0;", i))
		for j := 0; j < functionsPerFile; j++ {
			id := i*functionsPerFile + j
			body.WriteString(fmt.Sprintf("s+=f_%d();", id))
		}
		body.WriteString("return s;}\n")
		if err := os.WriteFile(path, []byte(body.String()), 0o600); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, nil
}
