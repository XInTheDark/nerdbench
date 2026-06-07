# Stockfish

## Purpose

Stockfish is the chess-engine workload candidate. It is valuable because `stockfish bench` is well-known, CPU-heavy, and naturally supports thread counts.

## Source

- Upstream: https://github.com/official-stockfish/Stockfish

## License

GPL-3.0.

If Stockfish is embedded in the default NerdBench binary, the clean project-level choice is to license NerdBench as GPL-3.0-or-later and ship complete corresponding source and build scripts.

## Build Notes

- Build as a static worker binary in GitHub Actions.
- Pin the upstream revision in `third_party/sources.lock`.
- Avoid CPU-specific builds that make results unfair across hosts.
- Record build flags in release metadata.

## Run Modes

- Single-core: run `stockfish bench` with one thread.
- Multi-core: run `stockfish bench` with the selected thread count.
- Use three measured runs and score the median.

## Metric

- Primary metric: nodes per second.
- Score direction: higher is better.

## Scoring

This is one test module. Use the median nodes-per-second result from measured runs as the normalized metric for both single and multi mode.
