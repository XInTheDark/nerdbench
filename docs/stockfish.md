# Stockfish

## Purpose

Stockfish is the chess-engine workload candidate. It is valuable because `stockfish speedtest` is CPU-heavy, runtime-controlled, and naturally supports thread counts.

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

- Single-core: run `stockfish speedtest 1 128 <seconds>`.
- Multi-core: run `stockfish speedtest <threads> 128 <seconds>`.
- Use the profile runtime budget for `<seconds>`.

## Metric

- Primary metric: nodes per second.
- Score direction: higher is better.

## Scoring

This is one test module. Use the nodes-per-second result from the budgeted `speedtest` run as the normalized metric for both single and multi mode.
