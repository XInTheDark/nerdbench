# C-Ray

## Purpose

C-Ray is the initial floating-point rendering workload candidate. It is simple, auditable, and useful as a ray-tracing style CPU workload.

## Source

- Upstream: https://github.com/vkoskiv/c-ray
- OpenBenchmarking profile reference: https://openbenchmarking.org/test/pts/c-ray

## License

MIT.

Keep the upstream copyright and license notice in `THIRD_PARTY_LICENSES.txt`.

## Build Notes

- Build as a static worker binary in GitHub Actions.
- Pin the upstream revision in `third_party/sources.lock`.
- Prefer deterministic embedded scene input or generated scene input.
- Current implementation pins `7f67117f341e26a748ceb7cd5746ce6a913f8a68`.
- `scripts/build-workers.sh` currently builds and embeds C-Ray for the local host target.
- NerdBench generates a tiny primitive-only scene at runtime and passes it to the embedded C-Ray binary.

## Run Modes

- Single-core: run with one render thread.
- Multi-core: run with `--threads N` or equivalent.

## Metric

- Primary metric: paths/rays per second, calculated as `width * height * samples / elapsed_seconds` for a fixed generated scene.
- Score direction: higher is better for throughput, lower is better for elapsed time before normalization.

## Scoring

This is one test module. Use the fixed-scene throughput metric as the normalized metric for both single and multi mode.
