# TinyCC Compile

## Purpose

TinyCC is the self-contained compilation workload candidate. It allows NerdBench to include a code-compilation-style benchmark without depending on a compiler installed on the target machine.

This must be labeled honestly as TinyCC compile throughput, not GCC, LLVM, kernel, or general build performance.

## Source

- Upstream: https://github.com/TinyCC/tinycc

## License

LGPL-2.1.

Keep TinyCC as a separate worker executable rather than linking it into the Go wrapper. Include license notices and source/build information in the release materials.

## Build Notes

- Build and embed the `tcc` executable as the worker binary.
- Pin the upstream revision in `third_party/sources.lock`.
- Generate a fixed C corpus at runtime and compile it to object files with the embedded `tcc` worker.
- Avoid writing large outputs to disk; use a private temp directory and clean it up.

## Run Modes

- Single-core: compile the fixed corpus sequentially.
- Multi-core: run independent compile workers with the selected worker count.

## Metric

- Primary metric: files per second, lines per second, or inverse elapsed time for a fixed corpus.
- Score direction: higher is better for throughput, lower is better for elapsed time before normalization.

## Scoring

This is one test module. Use fixed-corpus compile throughput as the normalized metric for both single and multi mode.
