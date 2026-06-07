# zstd

## Purpose

zstd is the compression workload candidate. It measures compression, decompression, memory bandwidth, cache behavior, and practical CPU throughput.

## Source

- Upstream: https://github.com/facebook/zstd

## License

BSD-style license.

Keep the upstream copyright and license notice in `THIRD_PARTY_LICENSES.txt`.

## Build Notes

- Build as a static worker binary in GitHub Actions.
- Pin the upstream revision in `third_party/sources.lock`.
- Use deterministic embedded or generated corpora.
- Keep corpora small enough that the release binary does not become unreasonable.

## Run Modes

- Single-core: run zstd benchmark with one thread.
- Multi-core: run zstd benchmark with the selected thread count.

## Metric

- Primary metric: geometric mean of compression and decompression MB/s.
- Score direction: higher is better.

## Scoring

This is one test module. Normalize the compression/decompression throughput metric into one subscore for each enabled mode.
