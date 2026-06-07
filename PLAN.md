# NerdBench Plan

## Goal

Build an open-source, single-binary benchmark that can replace Geekbench in YABS-style scripts without runtime dependencies, proprietary downloads, accounts, or rate limits.

The default release artifact should work like this:

```sh
curl -fsSL https://example.com/bench.sh | bash -s -- -o nerdbench.json
```

Success criteria:

- One downloaded executable per supported OS/arch.
- No required packages on the target machine.
- Live progress while the benchmark is running.
- Single-core and multi-core scores.
- Final human-readable output plus machine-readable JSON.
- Default runtime around 15 minutes on a high-end CPU, with a hard cap around 30 minutes on low-end systems.
- Deterministic, versioned scoring so results remain comparable across releases.

## Product Decisions

Use Go for the wrapper.

Reasons:

- Produces a static Linux binary easily with `CGO_ENABLED=0`.
- `//go:embed` is simple and reliable.
- The standard library is enough for subprocess execution, JSON, hashing, temp files, progress rendering, and timeouts.
- Cross-compilation for the wrapper is straightforward.

Do not bundle Phoronix Test Suite.

Use upstream benchmark projects directly, pin their source revisions, build worker binaries in CI, embed those workers into the Go executable, then run them under a strict manifest.

Do not use host compilers or host libraries during benchmark execution.

That rules out real GCC/kernel/LLVM compilation as a default workload. A compiler benchmark can exist, but it must use an embedded compiler and a fixed embedded corpus.

## Licensing Position

Recommended project license: GPL-3.0-or-later.

Reason: Stockfish is GPL-3.0. If the release binary embeds Stockfish, licensing the whole NerdBench repository as GPL-3.0-or-later is the simplest, clearest path. The whole repo will be open source, so this is a good fit.

Release obligations:

- Add root `LICENSE` containing GPL-3.0-or-later.
- Add a short license notice to project-owned source files or a clear repository-level notice in `README.md`.
- Ship complete corresponding source for the wrapper, build scripts, benchmark patches, pinned source manifests, and embedded worker build process.
- Ship `THIRD_PARTY_LICENSES.txt` in release artifacts.
- Keep per-benchmark documentation in `docs/` with source location, license, build notes, and scoring notes.
- Do not add extra restrictions to the GPL-covered release artifacts.

Permissive or public-domain upstream code can be included in a GPL-licensed project when the licenses are GPL-compatible. Those components keep their original notices, but the combined distributed work is GPL-covered.

This is not legal advice; it is an engineering plan to avoid an obvious license mismatch.

## YABS Compatibility

NerdBench being GPL-3.0-or-later should not block YABS integration.

YABS is a shell script licensed under WTFPL, and it already downloads and runs external binaries such as Geekbench, fio, and iperf. The important distinction is that YABS should call or download NerdBench as a separate program, not copy NerdBench source into the YABS script or merge it into one combined executable.

Recommended integration model:

- YABS downloads the appropriate NerdBench release binary or calls `scripts/bench.sh`.
- NerdBench runs as a separate process.
- YABS parses NerdBench JSON output.
- YABS does not vendor NerdBench source or embedded worker binaries inside `yabs.sh`.

That is the same general shape as YABS downloading Geekbench today, except NerdBench will be open source and redistributable under GPL terms. A WTFPL script can invoke a GPL program as a separate program. If YABS maintainers ever want to redistribute NerdBench binaries from the YABS repo or release bundle, they need to include the GPL license and corresponding source offer/source links for the NerdBench binary they redistribute.

## Benchmark Suite

The v1 suite should cover practical server CPU behavior, not just synthetic loops. Each workload must support:

- `single` mode using one worker thread.
- `multi` mode using all logical CPUs by default, with `--threads N` override.
- A parser that returns a throughput or latency metric.
- A timeout.
- A small smoke-test mode for CI.

Recommended v1 workloads:

| Workload | Source | License | Measures | Single-core | Multi-core | Score metric |
| --- | --- | --- | --- | --- | --- | --- |
| sysbench CPU | `akopytov/sysbench` | GPL-2.0-or-later | Integer/control workload | `--threads=1` | `--threads=N` | events/sec |
| C-Ray | `vkoskiv/c-ray` | MIT | Floating point, ray/path tracing, memory locality | `threads=1` | `threads=N` | rays/sec or inverse render time |
| Stockfish | `official-stockfish/Stockfish` | GPL-3.0 | Chess search, branch prediction, integer-heavy workload | `stockfish bench` with one thread | `stockfish bench` with N threads | nodes/sec median of 3 |
| SQLite speedtest1 | SQLite `tool/speedtest1.c` | Public domain | DB engine CPU, allocator, cache, mixed branch/memory behavior | one in-memory DB | N independent DB workers | tests/sec or inverse elapsed time |
| OpenSSL speed | OpenSSL 3.x | Apache-2.0 | AES, SHA, ChaCha, public-key crypto | `openssl speed -elapsed -seconds S` | `openssl speed -multi N` | bytes/sec and ops/sec |
| zstd bench | `facebook/zstd` | BSD-style | Compression, decompression, memory bandwidth | `-T1` | `-T0` or `-T N` | MB/sec geometric mean |
| ML kernel | `ggml-org/llama.cpp` / ggml | MIT | FP32/FP16/int8 tensor math and cache behavior | one thread | N threads | tokens/sec equivalent or GFLOP/sec |
| TinyCC compile | `TinyCC/tinycc` | LGPL-2.1 | Self-contained code parsing/codegen | compile fixed C corpus to object | N independent compile workers | files/sec or lines/sec |

Notes:

- sysbench replaces CoreMark to avoid CoreMark's EEMBC acceptable-use/trademark issues and to provide a more familiar server benchmark. Use only sysbench's built-in CPU workload; avoid sysbench memory, threads, and OLTP as default v1 workloads.
- C-Ray gives a simple floating-point/rendering anchor and is easy to explain.
- Stockfish is valuable because people recognize `stockfish bench`, but it drives the project license decision.
- SQLite should run primarily in-memory. YABS already covers disk; NerdBench should not turn the DB workload into a noisy storage benchmark.
- OpenSSL should use a fixed subset such as `sha256`, `aes-256-gcm`, `chacha20-poly1305`, `rsa2048`, and `ed25519` if supported by the selected OpenSSL version.
- The ML workload should start as a CPU-only ggml tensor benchmark with generated deterministic weights. Do not require downloading a model.
- TinyCC compile is the compromise for "code compilation" under the single-binary/no-dependency constraint. It measures compiler throughput, not GCC/LLVM build performance.

## CPU Optimization Policy

Geekbench-style CPU feature use is acceptable. Modern CPUs should be allowed to benefit from instructions such as AVX, AVX2, AVX-512, AES-NI, NEON, or SVE where a benchmark legitimately uses them.

The release build must still be reproducible. Do not use `-march=native` in GitHub Actions release builds.

Recommended v1 build model:

- Build one NerdBench wrapper per OS/arch:
  - `nerdbench-linux-amd64`
  - `nerdbench-linux-arm64`
- Inside that wrapper, embed one worker binary per benchmark per required architecture.
- For benchmarks with their own runtime CPU dispatch, build the normal upstream release worker and let that benchmark select optimized code internally.
- For benchmarks that require compile-time CPU targets, start with the portable worker only. Add feature-tier workers later only when the benchmark has clear value from them and the scoring policy is updated.

Initial worker set:

```text
nerdbench-linux-amd64
  sysbench-linux-amd64
  c-ray-linux-amd64
  stockfish-linux-amd64
  sqlite-speedtest-linux-amd64
  openssl-speed-linux-amd64
  zstd-linux-amd64
  ggml-ml-kernel-linux-amd64
  tinycc-compile-linux-amd64

nerdbench-linux-arm64
  sysbench-linux-arm64
  c-ray-linux-arm64
  stockfish-linux-arm64
  sqlite-speedtest-linux-arm64
  openssl-speed-linux-arm64
  zstd-linux-arm64
  ggml-ml-kernel-linux-arm64
  tinycc-compile-linux-arm64
```

Selection model:

- `scripts/bench.sh` selects only the top-level NerdBench binary by OS/arch.
- The NerdBench wrapper selects the embedded worker matching its own OS/arch.
- v1 does not need separate external downloads for AVX/non-AVX workers.
- If feature-tier workers are added later, the wrapper performs CPU feature detection and selects the best supported embedded worker. That change requires a new `score_version`.

## Runtime Budget

Default profile: `standard`.

Target wall-clock budget:

- High-end machine: about 15 minutes.
- Low-end machine: hard cap around 30 minutes.

Profiles:

| Profile | Target | Intended use |
| --- | ---: | --- |
| `smoke` | 30-90 seconds | CI and quick install validation |
| `quick` | 5-7 minutes | Fast VPS spot-check |
| `standard` | 15 minutes high-end, 30 minute cap low-end | YABS default candidate |
| `extended` | 30-60 minutes | Better stability for reviewers |

Budget strategy:

1. Run a short calibration pass for each workload.
2. Estimate iterations, scene size, seconds, or repetition count needed for the profile.
3. Run the measured pass with a per-workload budget and timeout.
4. Use median for repeated latency-like runs.
5. Use geometric mean for aggregate scores.
6. Abort the whole run cleanly if the global timeout is reached.

Initial `standard` budget:

| Workload | Single | Multi | Notes |
| --- | ---: | ---: | --- |
| sysbench | 60s | 60s | CPU test only |
| C-Ray | 90s | 90s | Increase samples/scene only through manifest |
| Stockfish | 3 x 45s | 3 x 45s | Median nodes/sec |
| SQLite | 90s | 90s | In-memory default, temp-file optional subtest |
| OpenSSL | 120s | 120s | Several algorithms, short fixed windows |
| zstd | 120s | 120s | Embedded/generated corpus |
| ML kernel | 120s | 120s | Deterministic matrix sizes |
| TinyCC | 60s | 60s | Compile-to-object only |

The exact numbers should be adjusted after baseline runs. The important part is that runtime is profile-driven and capped, not a pile of fixed commands that accidentally take an hour on small VPSes.

## Architecture

Proposed repository layout:

```text
cmd/nerdbench/
  main.go
internal/assets/
  embed.go
internal/bench/
  manifest.go
  sysbench.go
  cray.go
  stockfish.go
  sqlite.go
  openssl.go
  zstd.go
  ggml.go
  tinycc.go
internal/runner/
  extract.go
  process.go
  timeout.go
  threads.go
internal/progress/
  progress.go
  text.go
  ndjson.go
internal/results/
  schema.go
  score.go
  baseline.go
  baselines/
    2026-xx.json
calibration/
  README.md
  runs/
    .gitkeep
  baselines/
    .gitkeep
docs/
  sysbench.md
  c-ray.md
  stockfish.md
  sqlite-speedtest.md
  openssl-speed.md
  zstd.md
  ggml-ml-kernel.md
  tinycc-compile.md
scripts/
  bench.sh
  calibrate.sh
  fetch-third-party.sh
  build-workers.sh
  smoke.sh
third_party/
  sources.lock
  patches/
  licenses/
build/
  workers/
  embedded/
.github/workflows/
  ci.yml
  release.yml
```

Core concepts:

- `Benchmark` interface: `Name()`, `Modes()`, `Calibrate()`, `Run()`, `Parse()`.
- `Manifest`: pinned worker asset names, SHA256 hashes, license metadata, command templates, timeout budgets, scoring baseline keys.
- `Runner`: extracts one worker asset to a private temp directory, verifies hash, runs with context timeout, captures stdout/stderr, deletes temp files.
- `Progress`: emits start/update/finish/error events as benchmarks run.
- `Scorer`: converts raw metrics into normalized scores using a versioned baseline.
- `calibration/`: stores raw calibration runs and generated baseline candidate JSON before a baseline is promoted into `internal/results/baselines/`.
- `docs/`: one Markdown file per benchmark with basic purpose, upstream source URL, license, build notes, run modes, parsed metric, and scoring notes.

The wrapper should run benchmarks sequentially by default. The benchmark itself controls thread count in `multi` mode. This avoids multiple workloads fighting each other and producing noisy scores.

## CLI

Binary name: `nerdbench`.

Initial CLI:

```text
nerdbench run [flags]

Flags:
  --profile smoke|quick|standard|extended   default: standard
  --threads N                               default: logical CPU count
  --single-only
  --multi-only
  --bench name[,name...]
  --skip name[,name...]
  --format text|json                        default: text
  --progress text|ndjson|none               default: text
  -o, --output PATH                         write full JSON result
  --timeout 30m                             global hard timeout
  --tmpdir PATH                             override extraction directory
  --no-cleanup                              keep extracted workers for debugging
  --version
```

Output rules:

- Progress goes to stderr by default.
- Human summary goes to stdout for `--format text`.
- Final JSON goes to stdout for `--format json`.
- `-o result.json` writes the full JSON result regardless of stdout format.
- Exit code is non-zero if required workloads fail or the global timeout is hit.
- Partial JSON should still be written on failure with `"status": "failed"` and per-workload errors.

## JSON Schema

The JSON should be stable from v1.0 onward.

Shape:

```json
{
  "schema_version": 1,
  "nerdbench_version": "0.1.0",
  "score_version": "2026-06-07",
  "profile": "standard",
  "started_at": "2026-06-07T03:00:00Z",
  "duration_ms": 912345,
  "status": "ok",
  "system": {
    "os": "linux",
    "arch": "amd64",
    "kernel": "6.8.0",
    "cpu_model": "AMD EPYC ...",
    "logical_cpus": 8,
    "memory_bytes": 4294967296,
    "virtualization": "kvm"
  },
  "provenance": {
    "manifest_hash": "sha256:...",
    "baseline_hash": "sha256:...",
    "benchmarks": [
      {
        "name": "stockfish",
        "source": "https://github.com/official-stockfish/Stockfish",
        "revision": "abcdef123456",
        "license": "GPL-3.0",
        "worker_sha256": "sha256:...",
        "compiler": "gcc 14.2.0",
        "build_flags": "-O3 ...",
        "command": "stockfish bench ..."
      }
    ]
  },
  "scores": {
    "single": 1234,
    "multi": 6789
  },
  "benchmarks": [
    {
      "name": "stockfish",
      "mode": "single",
      "status": "ok",
      "threads": 1,
      "duration_ms": 135000,
      "metric": {
        "name": "nodes_per_second",
        "value": 12345678,
        "unit": "nps"
      },
      "score": 1234,
      "raw": {
        "runs": [123, 124, 122]
      },
      "log": {
        "stdout_tail": "last useful parsed lines only",
        "stderr_tail": ""
      }
    }
  ],
  "errors": []
}
```

Do not include huge raw logs by default. Keep enough raw parsed data to audit the score, plus compact provenance and short stdout/stderr tails for failed or suspicious runs.

Default JSON should include:

- Source revision, license, worker hash, compiler, build flags, selected command, manifest hash, and baseline hash.
- Parsed raw metrics used for scoring.
- Short log tails, capped to a small size such as 4-8 KiB per failed benchmark and less for successful benchmarks.

Full worker stdout/stderr should stay out of default JSON. Add `--debug` later if full stdout/stderr capture is needed.

## Scoring

Use normalized benchmark scores.

For throughput metrics:

```text
subscore = 1000 * (measured_throughput / baseline_throughput)
```

For latency metrics:

```text
subscore = 1000 * (baseline_latency / measured_latency)
```

Aggregation:

- Each test module produces one normalized single subscore and one normalized multi subscore when both modes are enabled.
- Final single score: geometric mean of successful single-mode test module subscores.
- Final multi score: geometric mean of successful multi-mode test module subscores.
- Do not calculate weighted score groups or expose additional grouped score fields.

Baseline policy:

- Pick one public baseline worker before v1, for example a specific low-cost VPS SKU or owned bare-metal host.
- Run `standard` at least 5 times on an idle system using `scripts/calibrate.sh`.
- Store raw calibration result files under `calibration/runs/<score_version>/`.
- Generate a reviewed baseline candidate under `calibration/baselines/<score_version>.json`.
- Promote the reviewed baseline into `internal/results/baselines/<score_version>.json`.
- Never mutate a promoted baseline file after release.
- If benchmark commands, compiler flags, source revisions, or scoring inputs change, bump `score_version`.

## Calibration Workflow

Calibration is how NerdBench decides what "score 1000" means for a score version.

The workflow should be explicit and reproducible:

```sh
scripts/calibrate.sh --score-version 2026-06-07 --runs 5
```

To also set the generated candidate as the runtime baseline after validation:

```sh
scripts/calibrate.sh --score-version 2026-06-07 --runs 5 --set-baseline
```

`scripts/calibrate.sh` responsibilities:

1. Create `calibration/runs/<score_version>/`.
2. Run `scripts/bench.sh --profile standard --format json -o calibration/runs/<score_version>/run-001.json`.
3. Repeat until the requested run count is complete.
4. Validate that every run has the same `nerdbench_version`, `score_version`, workload list, profile, and system identity.
5. Extract raw metrics from each run.
6. Compute median baseline metrics per benchmark, mode, and metric.
7. Write `calibration/baselines/<score_version>.json`.
8. If `--set-baseline` is passed, validate that `internal/results/baselines/<score_version>.json` does not already exist, then copy the generated candidate there.
9. If `--set-baseline` is not passed, print the exact command to rerun with `--set-baseline`.

Default behavior should not set the runtime baseline. `--set-baseline` is an explicit review step because bad calibration data would permanently distort public scores.

Suggested calibration command:

```sh
scripts/calibrate.sh \
  --score-version 2026-06-07 \
  --runs 7 \
  --profile standard \
  --out-dir calibration/runs/2026-06-07
```

Suggested output layout:

```text
calibration/
  runs/
    2026-06-07/
      run-001.json
      run-002.json
      run-003.json
      run-004.json
      run-005.json
      run-006.json
      run-007.json
      summary.json
  baselines/
    2026-06-07.json
internal/results/baselines/
  2026-06-07.json
```

Baseline candidate JSON shape:

```json
{
  "score_version": "2026-06-07",
  "profile": "standard",
  "source_runs": [
    "calibration/runs/2026-06-07/run-001.json",
    "calibration/runs/2026-06-07/run-002.json"
  ],
  "system": {
    "provider": "example-vps",
    "instance_type": "baseline-worker",
    "cpu_model": "Example CPU",
    "logical_cpus": 4
  },
  "metrics": [
    {
      "benchmark": "stockfish",
      "mode": "single",
      "metric": "nodes_per_second",
      "unit": "nps",
      "baseline_value": 1234567,
      "statistic": "median",
      "run_values": [1220000, 1234567, 1250000]
    }
  ]
}
```

`calibration/runs/` should normally be gitignored except for `.gitkeep` and selected published calibration evidence. `calibration/baselines/` can keep reviewed candidates. `internal/results/baselines/` is the source of truth used by the compiled binary.

## Build And Release

GitHub Actions matrix:

- `linux/amd64`
- `linux/arm64`

Possible later targets:

- `linux/arm/v7`
- `freebsd/amd64`
- `darwin/arm64` for local development only, not YABS default.

CI stages:

1. Fetch pinned upstream sources from `third_party/sources.lock`.
2. Apply small local patches from `third_party/patches`.
3. Build static worker binaries for each target.
4. Run worker smoke tests under native or QEMU execution.
5. Generate `internal/assets/embed.go` asset list and manifest hashes.
6. Build static Go wrapper with embedded workers.
7. Run `nerdbench run --profile smoke --format json`.
8. Upload release assets:
   - `nerdbench-linux-amd64`
   - `nerdbench-linux-arm64`
   - `bench.sh`
   - `SHA256SUMS`
   - `THIRD_PARTY_LICENSES.txt`
   - source archive or source-fetch manifest sufficient for GPL compliance.

Use musl where practical for C/C++ workers. If a worker cannot be built cleanly with musl, spike it early and either patch it, replace it, or make it optional.

## Downloader Script

`scripts/bench.sh` should be tiny and auditable.

Responsibilities:

- Detect OS with `uname -s`.
- Detect arch with `uname -m`.
- Map common arch names:
  - `x86_64` / `amd64` -> `linux-amd64`
  - `aarch64` / `arm64` -> `linux-arm64`
- Download the matching release binary.
- Download `SHA256SUMS`.
- Verify checksum when `sha256sum` or `shasum -a 256` exists.
- `chmod +x`.
- Execute the binary with all passed arguments.

Example target UX:

```sh
curl -fsSL https://raw.githubusercontent.com/OWNER/nerdbench/main/scripts/bench.sh | bash
curl -fsSL https://raw.githubusercontent.com/OWNER/nerdbench/main/scripts/bench.sh | bash -s -- --profile quick -o result.json
```

The script is allowed to depend on `curl` or `wget`, because YABS-style install scripts already assume one network fetch tool. The downloaded benchmark executable itself should not depend on anything.

## Progress Streaming

Default human progress should be compact:

```text
[1/8] sysbench single running... 42s elapsed
[1/8] sysbench single done: 183421 events/s, score 1120
[2/8] sysbench multi running... threads=8
```

For machine consumers:

```sh
nerdbench run --progress ndjson -o result.json
```

NDJSON event examples:

```json
{"event":"bench_started","name":"stockfish","mode":"single","threads":1}
{"event":"bench_progress","name":"stockfish","mode":"single","elapsed_ms":30000}
{"event":"bench_finished","name":"stockfish","mode":"single","score":1234}
```

Progress should be emitted by the wrapper, not by trusting worker output. Worker stdout can be noisy and inconsistent.

## Verification

Required tests:

- Unit tests for every parser using frozen sample outputs.
- Unit tests for score math.
- Unit tests for arch/OS detection in `bench.sh` where practical.
- Calibration script dry run that writes multiple JSON files and a baseline candidate.
- Smoke integration test for each embedded worker.
- Full `standard` run on at least one amd64 and one arm64 Linux machine before first tagged release.

Manual release checklist:

- Run `nerdbench run --profile standard -o result.json`.
- Confirm all workers report `status: ok`.
- Confirm no temp files remain.
- Confirm `--format json` prints valid JSON and no progress noise to stdout.
- Confirm `-o` writes partial JSON on forced timeout.
- Confirm `bench.sh` downloads and verifies the correct asset on amd64 and arm64.
- Confirm `scripts/calibrate.sh --runs 3 --profile standard` writes raw runs and a baseline candidate on the selected baseline worker.

## Risks

Licensing:

- Stockfish makes GPL compatibility unavoidable if embedded in the default binary.
- LGPL TinyCC redistribution requires license notices and source availability. Keep TinyCC as a separate worker executable, not linked into Go.
- sysbench is GPL-2.0-or-later, which is compatible with the planned GPL-3.0-or-later NerdBench release. Include sysbench source/build information and notices with releases.

Binary size:

- Embedding OpenSSL, Stockfish, zstd, SQLite, ggml, and scenes/corpora may produce a large release binary.
- Compress embedded assets and keep corpora small.
- Prefer generated deterministic input where it produces representative work.

Runtime stability:

- VPS hosts are noisy.
- Thermal throttling and steal time can skew results.
- Use repeated runs and medians where cost-effective.
- Include system metadata, but do not try to "correct" scores automatically.

Multi-core fairness:

- Some workloads scale better than others.
- Multi score should be its own score, not just single score multiplied by thread count.
- Cap threads only when a workload becomes unstable at extreme counts.

Compilation benchmark honesty:

- Embedded TinyCC is not GCC/LLVM.
- Label it clearly as `tinycc_compile`, not `code_build`, and do not oversell it.

ML benchmark honesty:

- A ggml tensor kernel is not full LLM inference.
- Label it as `ml_tensor` unless a real embedded model and stable inference path are added later.

## Milestones

### Milestone 0: Repo Scaffold

- Initialize Go module.
- Add basic CLI.
- Add result schema.
- Add progress event model.
- Add `bench.sh`.
- Add `scripts/calibrate.sh` skeleton.
- Add CI skeleton.

Exit criteria:

- `go test ./...` passes.
- `nerdbench run --profile smoke --format json` works with one placeholder internal workload.

### Milestone 1: Worker Build Pipeline

- Add `third_party/sources.lock`.
- Add `scripts/fetch-third-party.sh`.
- Add `scripts/build-workers.sh`.
- Build amd64 and arm64 static workers for sysbench, C-Ray, SQLite, zstd.
- Current implementation has host-target worker generation for C-Ray and SQLite; linux/arm64 cross-worker builds remain pending.
- Embed workers into Go binary.

Exit criteria:

- CI produces `nerdbench-linux-amd64` and `nerdbench-linux-arm64`.
- Smoke profile runs at least four real workloads.

### Milestone 2: Full v1 Workload Set

- Add Stockfish if project license is GPL-compatible.
- Add OpenSSL speed.
- Add ggml tensor workload.
- Add TinyCC compile workload if build and license packaging are clean.

Exit criteria:

- `standard` profile runs all selected v1 workloads.
- Every parser has frozen sample output tests.
- Failures produce partial JSON.

### Milestone 3: Scoring

- Pick baseline system.
- Run repeated baseline measurements with `scripts/calibrate.sh`.
- Generate `calibration/baselines/<score_version>.json`.
- Promote and commit immutable `internal/results/baselines/<score_version>.json`.
- Implement score normalization and weighted geometric means.

Exit criteria:

- Result JSON includes final single and multi scores.
- Changing scoring data requires a `score_version` change.

### Milestone 4: YABS Integration Candidate

- Harden `bench.sh`.
- Document stdout/stderr behavior.
- Add release checksums and license bundle.
- Produce tagged pre-release.
- Test from a clean VPS with only `curl` or `wget`.

Exit criteria:

- One-command install/run works on amd64 and arm64.
- `-o result.json` works.
- Runtime lands within the target budget.
- Output is stable enough for YABS maintainers to parse.

## References To Verify During Implementation

- sysbench: https://github.com/akopytov/sysbench
- C-Ray: https://github.com/vkoskiv/c-ray
- C-Ray OpenBenchmarking profile: https://openbenchmarking.org/test/pts/c-ray
- Stockfish: https://github.com/official-stockfish/Stockfish
- SQLite public domain source: https://www.sqlite.org/about.html
- OpenSSL license: https://www.openssl-library.org/source/license/
- OpenSSL speed docs: https://docs.openssl.org/master/man1/openssl-speed/
- zstd: https://github.com/facebook/zstd
- llama.cpp / ggml: https://github.com/ggml-org/llama.cpp
- llama-bench output formats, useful for parser ideas: https://github.com/ggml-org/llama.cpp/blob/master/tools/llama-bench/README.md
- TinyCC: https://github.com/TinyCC/tinycc
