# sysbench

## Purpose

sysbench is the integer/control workload candidate. It replaces CoreMark to avoid CoreMark's benchmark naming and acceptable-use complications while using a benchmark that server users already recognize.

Use only sysbench's built-in CPU test. Do not use sysbench memory, threads, or OLTP as default v1 workloads; SQLite already covers the database test module and other test modules cover memory-heavy behavior.

## Source

- Upstream: https://github.com/akopytov/sysbench

## License

GPL-2.0-or-later.

This is compatible with the planned GPL-3.0-or-later NerdBench release. Keep upstream license notices, source provenance, build scripts, and any local patches in the release source materials.

## Build Notes

- Build as a self-contained worker binary in GitHub Actions.
- Pin the upstream revision in `third_party/sources.lock`.
- Store any required local patches under `third_party/patches/sysbench/`.
- Linux builds must fail if the produced worker is dynamically linked.

## Run Modes

- Single-core: run `sysbench cpu --threads=1 --time=S run`.
- Multi-core: run `sysbench cpu --threads=N --time=S run`.

## Metric

- CPU primary metric: events per second.
- Score direction: higher is better for throughput metrics.

## Scoring

This is one test module. Normalize CPU events per second into one single-mode subscore and one multi-mode subscore.
