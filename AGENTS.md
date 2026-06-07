# AGENTS.md

Project context for future Codex runs in this repo.

## Project Goal

NerdBench is an open-source, single-binary CPU benchmark intended for YABS-style server benchmarking. It should act like a Geekbench alternative from a user's point of view: download one executable for the machine, run it, stream progress, and produce text or JSON results.

The target release platforms are Linux amd64 and Linux arm64. The local development host may be macOS.

## Current Architecture

- Go wrapper binary in `cmd/nerdbench`.
- Test module definitions and runners in `internal/bench`.
- Result schema and scoring in `internal/results`.
- Progress output in `internal/progress`.
- System metadata in `internal/system`.
- Worker extraction/execution helpers in `internal/runner`.
- Embedded worker asset registry in `internal/assets`.
- Worker fetch/build scripts in `scripts/`.
- Calibration outputs in `calibration/`.
- One Markdown doc per test module in `docs/`.

The final release should embed static or self-contained third-party worker executables. At runtime, the Go wrapper extracts the matching worker to a private temp directory, verifies its hash, runs it, parses output, scores it against the promoted baseline, and cleans up.

## Terminology

Use "test module" for each benchmark. Do not introduce grouped score terminology. The result JSON should expose only overall single and multi scores plus per-test-module entries.

Required scoring:

```text
subscore = 1000 * (measured_value / baseline_value)
```

Overall single and multi scores are geometric means of successful test module subscores.

## Current Implementation State

Implemented:

- CLI, JSON schema, text output, progress streaming, `-o` JSON file output.
- Calibration script with `--set-baseline`.
- Baseline loader for embedded `internal/results/baselines/*.json`.
- Embedded worker extraction and SHA256 verification.
- Real worker paths for C-Ray and SQLite speedtest.
- Internal smoke fallback for unfinished test modules.

Still pending:

- Real workers for sysbench, Stockfish, OpenSSL speed, zstd, ggml ML kernel, and TinyCC compile.
- Linux amd64 and Linux arm64 worker build pipeline.
- Real promoted baseline from a selected calibration machine.
- Complete license bundle and third-party notices.
- Release checksums and final YABS integration docs.

## Important Files

- `PLAN.md`: high-level design and decisions.
- `TASK.md`: detailed remaining work checklist.
- `.thinking`: persistent work notes. Read this after context compaction or when resuming a non-trivial task.
- `cmd/nerdbench/main.go`: CLI and run orchestration.
- `internal/bench/bench.go`: test module definitions and worker execution.
- `internal/results/score.go`: baseline loading and score aggregation.
- `scripts/calibrate.sh`: calibration and baseline promotion.
- `scripts/build-workers.sh`: local worker build and embed generation.
- `scripts/fetch-third-party.sh`: pinned source checkout.
- `scripts/bench.sh`: user-facing download/run helper.

## Working Rules

- Keep changes surgical.
- Do not add speculative scripts or extra release processes unless explicitly requested.
- Use `scripts/calibrate.sh --set-baseline` for baseline promotion.
- Do not hardcode release baseline values in Go.
- Do not commit raw calibration runs, local worker binaries, `dist/`, or generated embed files unless the release process explicitly says to.
- Use `python3`, not `python`.
- The host may be macOS, but release behavior must be validated on Linux.
- Prefer `rg` for text search. Use `ast-grep` for syntax-aware code searches when useful.

## Generated Files

These are normally ignored and may be regenerated:

- `internal/assets/generated_workers.go`
- `internal/assets/embedded/*`
- `third_party/src/`
- `build/workers/`
- `dist/`
- `calibration/runs/`
- `calibration/baselines/`

## Verification Commands

Basic verification:

```sh
go test ./...
scripts/smoke.sh
go run ./cmd/nerdbench run --profile smoke --format json --progress none -o /tmp/nerdbench-smoke.json
python3 -m json.tool /tmp/nerdbench-smoke.json >/dev/null
```

Worker verification:

```sh
scripts/build-workers.sh
go test ./...
go run ./cmd/nerdbench run --profile smoke --bench c-ray,sqlite-speedtest --format json --progress none -o /tmp/nerdbench-workers.json
python3 -m json.tool /tmp/nerdbench-workers.json >/dev/null
```

Terminology verification:

```sh
rg --hidden -n "cat[e]gor|Cat[e]gory" . -g '!third_party/**' -g '!dist/**' -g '!build/**' -g '!.git/**'
```

Expected result: no output.
