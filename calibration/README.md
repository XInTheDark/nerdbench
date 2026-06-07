# Calibration

Calibration runs define the raw baseline metrics used by a NerdBench score version.

Expected workflow:

```sh
scripts/calibrate.sh --score-version 2026-06-07 --runs 7 --profile standard
```

The script should:

1. Run NerdBench through `scripts/bench.sh`.
2. Write raw run JSON files into `calibration/runs/<score_version>/`.
3. Generate a reviewed baseline candidate in `calibration/baselines/<score_version>.json`.
4. Leave promotion into `internal/results/baselines/<score_version>.json` as an explicit review step.

Raw calibration runs may be large and noisy, so `calibration/runs/` should normally stay ignored except for selected published evidence.
