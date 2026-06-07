# ggml ML Kernel

## Purpose

ggml is the initial ML/tensor workload candidate. It should measure CPU tensor math and memory behavior without requiring a downloaded model.

This should be labeled as an ML tensor workload, not full LLM inference.

## Source

- Upstream: https://github.com/ggml-org/llama.cpp

## License

MIT.

Keep the upstream copyright and license notice in `THIRD_PARTY_LICENSES.txt`.

## Build Notes

- Prefer a small custom worker linked against ggml or an upstream benchmark tool if its output is stable.
- Build as a static worker binary in GitHub Actions.
- Pin the upstream revision in `third_party/sources.lock`.
- Generate deterministic tensors instead of embedding a model file.

## Run Modes

- Single-core: run tensor workload with one thread.
- Multi-core: run tensor workload with the selected thread count.

## Metric

- Primary metric: GFLOP/s, tensor operations per second, or a stable benchmark throughput metric.
- Score direction: higher is better.

## Scoring

This is one test module. Use the selected stable tensor-throughput metric as the normalized metric for both single and multi mode.
