# OpenSSL Speed

## Purpose

OpenSSL `speed` is the crypto workload candidate. It should cover hashing, symmetric crypto, and a small public-key subset without depending on host OpenSSL.

## Source

- Upstream: https://github.com/openssl/openssl
- License page: https://www.openssl-library.org/source/license/
- `openssl speed` docs: https://docs.openssl.org/master/man1/openssl-speed/

## License

Apache-2.0.

Keep the upstream license notice in `THIRD_PARTY_LICENSES.txt`.

## Build Notes

- Build OpenSSL as a static worker in GitHub Actions.
- Pin the upstream release tag or commit in `third_party/sources.lock`.
- Use a fixed algorithm subset for stable scoring.

## Run Modes

- Single-core: `openssl speed -elapsed -seconds S` for selected algorithms.
- Multi-core: `openssl speed -multi N` for selected algorithms.

## Metric

- Primary metrics: bytes per second for hash/symmetric crypto, operations per second for public-key crypto.
- Score direction: higher is better.

## Scoring

This is one test module. Normalize the fixed algorithm subset into one subscore for each enabled mode.
