# NerdBench

NerdBench is a **single-binary** CPU benchmark intended for server benchmarking.

It's essentially a Go wrapper for a bunch of test modules. These include: C-Ray, SQLite speedtest, sysbench CPU, Stockfish, OpenSSL speed, zstd, ggml ML kernel, and TinyCC compile. They are embedded inside the binary so you don't have to build anything. 

NerdBench reports both single-core and multi-core scores. The scores are based on a calibrated baseline of 1000. It can also write detailed results to a json file, allowing you to use it in other scripts.

## Run

```sh
go run ./cmd/nerdbench run --profile smoke
go run ./cmd/nerdbench run --profile smoke --format json -o result.json
```

Progress is written to stderr. Text or JSON results are written to stdout, and `-o` always writes the full JSON result file.

### Exit Codes

- Exit code `0`: all selected test modules passed.
- Exit code `1`: one or more test modules failed, or a usage error occurred.

Both text and JSON output return non-zero when any test module fails.

### Profiles

| Profile    | Target Runtime | Description |
|------------|----------------|-------------|
| `smoke`    | < 1 minute     | Quick CI validation with minimal workloads |
| `quick`    | ~ 1-2 minutes  | Short benchmark run |
| `standard` | ~ 5 minutes    | Full benchmark suite (default) |
| `extended` | ~ 20 minutes   | Extended workloads for thorough testing |

## JSON Output

The JSON result schema (version 1) contains:

- `status`: `"ok"` or `"failed"`
- `scores.single` / `scores.multi`: geometric mean of successful test module subscores
- `benchmarks[]`: per-test-module results with `status`, `metric`, `score`
- `errors[]`: error entries for failed test modules
- `provenance`: baseline hash and per-worker source, revision, license, SHA256

A failed test module appears in `benchmarks[]` with `status: "failed"` and no score, and is also listed in `errors[]`.

## Build Workers

```sh
scripts/build-workers.sh
go run ./cmd/nerdbench run --profile smoke --bench c-ray
```

`scripts/build-workers.sh` fetches third-party sources, builds worker binaries for the native host target, verifies Linux workers are self-contained, and generates embedded asset registration under `internal/assets/`.

## Cross-Build (Linux)

```sh
scripts/build-workers.sh   # native build for local target
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o dist/nerdbench-linux-amd64 ./cmd/nerdbench
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o dist/nerdbench-linux-arm64 ./cmd/nerdbench
```

Worker binaries must be built in a native environment for their target OS/arch. The release workflow uses QEMU-backed Linux arm64 containers for arm64 workers instead of cross-labeling host-built artifacts.

## Calibration

```sh
scripts/calibrate.sh --score-version 2026-06-07-dev --runs 3 --profile smoke
scripts/calibrate.sh --score-version 2026-06-07-dev --runs 3 --profile smoke --set-baseline
```

Raw calibration runs are written to `calibration/runs/<score_version>/`. Generated baseline candidates are written to `calibration/baselines/<score_version>.json`. `--set-baseline` copies the generated candidate into `internal/results/baselines/<score_version>.json` if it does not already exist.

## YABS Integration

NerdBench can be called as an external benchmark tool. For example, you can

1. Download the appropriate Linux binary via `scripts/bench.sh`
2. Parse JSON output (`--format json`) for programmatic consumption
3. Parse text output for human-readable display

Example:

```sh
curl -fsSL https://raw.githubusercontent.com/XInTheDark/nerdbench/main/bench.sh | sh -s -- --profile standard --format json --progress none -o /tmp/nerdbench.json
```

NerdBench is licensed under GPL-3.0-or-later. YABS may call NerdBench as a separate GPL program and parse its output, similar to how it calls other external benchmark tools.
