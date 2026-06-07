# Calibration

Calibration runs define the raw baseline metrics used by a NerdBench score version.

Expected workflow:

```sh
scripts/bench.sh --profile standard --format json --progress none -o /tmp/nerdbench-run-001.json
scripts/calibrate.sh --score-version 2026-06-07 --result /tmp/nerdbench-run-001.json
```

For multiple calibration runs, create multiple result JSON files and pass each one with `--result`.

The calibration flow should:

1. Run NerdBench separately and write raw result JSON files.
2. Generate a reviewed baseline candidate from those files in `calibration/baselines/<score_version>.json`.
3. Leave promotion into `internal/results/baselines/<score_version>.json` as an explicit review step.

Raw calibration runs may be large and noisy, so `calibration/runs/` should normally stay ignored except for selected published evidence.
