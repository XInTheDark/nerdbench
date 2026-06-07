package bench

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"time"

	"github.com/nerdbench/nerdbench/internal/assets"
	"github.com/nerdbench/nerdbench/internal/runner"
)

func (b Benchmark) runCRay(ctx context.Context, req RunRequest) (RunOutput, bool, error) {
	asset, ok := assets.FindWorker("c-ray", runtime.GOOS, runtime.GOARCH)
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
	width, height, samples, bounces := cRaySceneParams(req.Profile)
	if req.Profile != "smoke" {
		calibWidth, calibHeight, calibSamples, calibBounces := cRayCalibrationSceneParams()
		calib, err := runCRayScene(ctx, worker.Path, workDir, calibWidth, calibHeight, calibSamples, calibBounces, threads, "calibration-scene.json")
		if err != nil {
			return RunOutput{
				Duration:   calib.elapsed,
				StdoutTail: runner.Tail(calib.stdout, 4096),
				StderrTail: runner.Tail(calib.stderr, 4096),
			}, true, err
		}
		calibSeconds := calib.renderSeconds
		if calibSeconds <= 0 {
			calibSeconds = calib.elapsed.Seconds()
		}
		if calibSeconds > 0 {
			targetSamples := float64(calibSamples) * profileDuration(req.Profile).Seconds() / calibSeconds
			samples = maxInt(samples, int(math.Ceil(targetSamples)))
			width = calibWidth
			height = calibHeight
			bounces = calibBounces
		}
	}

	measured, err := runCRayScene(ctx, worker.Path, workDir, width, height, samples, bounces, threads, "scene.json")
	if err != nil {
		return RunOutput{
			Duration:   measured.elapsed,
			StdoutTail: runner.Tail(measured.stdout, 4096),
			StderrTail: runner.Tail(measured.stderr, 4096),
		}, true, err
	}
	renderSeconds := measured.renderSeconds
	if renderSeconds <= 0 {
		renderSeconds = measured.elapsed.Seconds()
	}
	targetSeconds := profileDuration(req.Profile).Seconds()
	if req.Profile != "smoke" && renderSeconds > 0 && renderSeconds < targetSeconds*0.8 {
		samples = maxInt(samples+1, int(math.Ceil(float64(samples)*targetSeconds/renderSeconds)))
		measured, err = runCRayScene(ctx, worker.Path, workDir, width, height, samples, bounces, threads, "scene.json")
		if err != nil {
			return RunOutput{
				Duration:   measured.elapsed,
				StdoutTail: runner.Tail(measured.stdout, 4096),
				StderrTail: runner.Tail(measured.stderr, 4096),
			}, true, err
		}
		renderSeconds = measured.renderSeconds
		if renderSeconds <= 0 {
			renderSeconds = measured.elapsed.Seconds()
		}
	}
	paths := float64(width * height * samples)
	value := paths / renderSeconds
	return RunOutput{
		Value:      value,
		Duration:   measured.elapsed,
		Iterations: uint64(paths),
		StdoutTail: runner.Tail(measured.stdout, 512),
		StderrTail: runner.Tail(measured.stderr, 512),
		Note:       "c-ray embedded worker",
	}, true, nil
}

func cRaySceneParams(profile string) (width, height, samples, bounces int) {
	switch profile {
	case "smoke":
		return 128, 128, 4, 4
	case "quick":
		return 256, 256, 16, 6
	case "extended":
		return 384, 384, 24, 8
	default:
		return 384, 384, 24, 8
	}
}

func cRayCalibrationSceneParams() (width, height, samples, bounces int) {
	return 384, 384, 24, 8
}

type cRayRunResult struct {
	renderSeconds float64
	elapsed       time.Duration
	stdout        string
	stderr        string
}

func runCRayScene(ctx context.Context, workerPath, workDir string, width, height, samples, bounces, threads int, sceneFile string) (cRayRunResult, error) {
	scene := cRayScene(width, height, samples, bounces, threads)
	scenePath := filepath.Join(workDir, sceneFile)
	if err := os.MkdirAll(filepath.Join(workDir, "output"), 0o700); err != nil {
		return cRayRunResult{}, err
	}
	if err := os.WriteFile(scenePath, []byte(scene), 0o600); err != nil {
		return cRayRunResult{}, err
	}

	start := time.Now()
	proc, err := runner.RunProcessInDir(ctx, workDir, workerPath, scenePath)
	elapsed := time.Since(start)
	return cRayRunResult{
		renderSeconds: parseCRaySeconds(proc.Stderr),
		elapsed:       elapsed,
		stdout:        proc.Stdout,
		stderr:        proc.Stderr,
	}, err
}

func cRayScene(width, height, samples, bounces, threads int) string {
	return fmt.Sprintf(`{
  "version": 1.0,
  "renderer": {
    "threads": %d,
    "samples": %d,
    "bounces": %d,
    "tileWidth": 16,
    "tileHeight": 16,
    "tileOrder": "fromMiddle",
    "outputFilePath": "output/",
    "outputFileName": "rendered",
    "fileType": "png",
    "count": 0,
    "width": %d,
    "height": %d
  },
  "display": { "isFullscreen": false, "windowScale": 1.0 },
  "camera": {
    "FOV": 30.0,
    "focalDistance": 0.7,
    "fstops": 0,
    "transforms": [
      { "type": "translate", "x": 0, "y": 0, "z": -1.5 }
    ]
  },
  "scene": {
    "ambientColor": {
      "type": "background",
      "down": { "r": 1.0, "g": 1.0, "b": 1.0 },
      "up": { "r": 0.5, "g": 0.7, "b": 1.0 }
    },
    "primitives": [
      {
        "type": "sphere",
        "instances": [{ "transforms": [{ "type": "translate", "x": 0, "y": 0, "z": 0 }] }],
        "material": { "type": "diffuse", "color": { "r": 0.9, "g": 0.2, "b": 0.2 } },
        "radius": 0.25
      }
    ],
    "meshes": []
  }
}
`, threads, samples, bounces, width, height)
}

var cRayFinishedRE = regexp.MustCompile(`Finished render in ([0-9.]+)(ms|s)`)

func parseCRaySeconds(log string) float64 {
	matches := cRayFinishedRE.FindStringSubmatch(log)
	if len(matches) != 3 {
		return 0
	}
	value, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0
	}
	if matches[2] == "ms" {
		return value / 1000
	}
	return value
}
