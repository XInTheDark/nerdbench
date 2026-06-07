#!/usr/bin/env sh
set -eu

score_version=""
runs="5"
profile="standard"
out_dir=""
set_baseline="false"

while [ "$#" -gt 0 ]; do
  case "$1" in
    --score-version) score_version="$2"; shift 2 ;;
    --runs) runs="$2"; shift 2 ;;
    --profile) profile="$2"; shift 2 ;;
    --out-dir) out_dir="$2"; shift 2 ;;
    --set-baseline) set_baseline="true"; shift ;;
    *) echo "unknown argument: $1" >&2; exit 1 ;;
  esac
done

if [ -z "$score_version" ]; then
  echo "--score-version is required" >&2
  exit 1
fi

case "$runs" in
  ''|*[!0-9]*) echo "--runs must be a positive integer" >&2; exit 1 ;;
esac
[ "$runs" -gt 0 ] || { echo "--runs must be > 0" >&2; exit 1; }

repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
out_dir="${out_dir:-$repo_root/calibration/runs/$score_version}"
candidate="$repo_root/calibration/baselines/$score_version.json"
baseline="$repo_root/internal/results/baselines/$score_version.json"

mkdir -p "$out_dir" "$repo_root/calibration/baselines" "$repo_root/internal/results/baselines"

i=1
while [ "$i" -le "$runs" ]; do
  run_file="$out_dir/run-$(printf '%03d' "$i").json"
  echo "calibration run $i/$runs -> $run_file" >&2
  "$repo_root/scripts/bench.sh" --profile "$profile" --format json --progress none -o "$run_file" >/dev/null
  i=$((i + 1))
done

python3 - "$score_version" "$profile" "$candidate" "$out_dir"/run-*.json <<'PY'
import json
import statistics
import sys
from pathlib import Path

score_version, profile, candidate, *run_files = sys.argv[1:]
docs = []
for path in run_files:
    with open(path, "r", encoding="utf-8") as f:
        docs.append((path, json.load(f)))

if not docs:
    raise SystemExit("no run files")

first = docs[0][1]
for path, doc in docs:
    if doc.get("profile") != profile:
        raise SystemExit(f"{path}: profile mismatch")
    if doc.get("status") != "ok":
        raise SystemExit(f"{path}: run status is not ok")

values = {}
for path, doc in docs:
    for b in doc.get("benchmarks", []):
        if b.get("status") != "ok":
            raise SystemExit(f"{path}: benchmark failed: {b.get('name')} {b.get('mode')}")
        metric = b.get("metric", {})
        key = (b.get("name"), b.get("mode"), metric.get("name"), metric.get("unit"))
        values.setdefault(key, []).append(float(metric.get("value", 0)))

metrics = []
for (benchmark, mode, metric, unit), vals in sorted(values.items()):
    metrics.append({
        "benchmark": benchmark,
        "mode": mode,
        "metric": metric,
        "unit": unit,
        "baseline_value": statistics.median(vals),
        "statistic": "median",
        "run_values": vals,
    })

out = {
    "score_version": score_version,
    "profile": profile,
    "source_runs": [str(Path(path)) for path, _ in docs],
    "system": first.get("system", {}),
    "metrics": metrics,
}

with open(candidate, "w", encoding="utf-8") as f:
    json.dump(out, f, indent=2)
    f.write("\n")
PY

echo "baseline candidate -> $candidate" >&2

if [ "$set_baseline" = "true" ]; then
  if [ -e "$baseline" ]; then
    echo "baseline already exists: $baseline" >&2
    exit 1
  fi
  cp "$candidate" "$baseline"
  echo "runtime baseline set -> $baseline" >&2
else
  echo "to set runtime baseline, rerun with --set-baseline" >&2
fi
