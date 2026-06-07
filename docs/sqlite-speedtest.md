# SQLite Speedtest1

## Purpose

SQLite `speedtest1` is the database workload candidate. It gives a practical mixed workload covering SQL execution, B-tree operations, memory allocation, cache behavior, and branch-heavy code.

NerdBench should run this primarily as an in-memory CPU benchmark. YABS already covers disk performance separately.

## Source

- Upstream: https://www.sqlite.org/src/
- SQLite about/license page: https://www.sqlite.org/about.html
- GitHub mirror used for pinned builds: https://github.com/sqlite/sqlite

## License

SQLite is public domain.

Keep source provenance in `THIRD_PARTY_LICENSES.txt` even though there is no conventional license notice requirement.

## Build Notes

- Build SQLite plus `tool/speedtest1.c` as a static worker binary in GitHub Actions.
- Pin the upstream source archive or Fossil checkout in `third_party/sources.lock`.
- Use fixed compile-time options for reproducibility.
- Current implementation pins Git mirror revision `ccc132c5be20ab5c755c97a08d06b1b592fef330` (`version-3.53.2`).
- `scripts/build-workers.sh` runs SQLite's generated-source build to produce `sqlite3.c`, then compiles `test/speedtest1.c` with the amalgamation.
- Current compile flags include `-O2 -DSQLITE_THREADSAFE=0 -DSQLITE_OMIT_LOAD_EXTENSION -DSQLITE_TEMP_STORE=3`.

## Run Modes

- Single-core: one in-memory database workload.
- Multi-core: run independent in-memory database workers and aggregate completed operations.

## Metric

- Primary metric: suites per second for a fixed `speedtest1` `main` test set.
- Single-core score uses `1 / TOTAL_seconds`.
- Multi-core score runs N independent workers and uses `N / max(TOTAL_seconds)`.
- Score direction: higher is better for throughput, lower is better for elapsed time before normalization.

## Scoring

This is one test module. Use the fixed in-memory speedtest throughput metric as the normalized metric for both single and multi mode.
