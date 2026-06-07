#!/usr/bin/env sh
set -eu

usage() {
  cat >&2 <<'EOF'
usage: scripts/calibrate.sh --score-version VERSION [--profile PROFILE] [--set-baseline] --result RESULT.json [...]
       scripts/calibrate.sh --score-version VERSION [--profile PROFILE] [--set-baseline] RESULT.json [...]

Create a baseline candidate from existing NerdBench JSON result files.

Generate result files separately, for example:
  scripts/bench.sh --profile standard --format json --progress none -o /tmp/nerdbench-run-001.json

Options:
  --score-version VERSION  baseline score version to write
  --result FILE           existing NerdBench JSON result; may be repeated
  --profile PROFILE       optional assertion; inferred from result JSON when omitted
  --set-baseline          copy candidate into internal/results/baselines/ if absent
  -h, --help              show this help
EOF
}

need_value() {
  if [ "$#" -lt 2 ]; then
    echo "$1 requires a value" >&2
    exit 1
  fi
}

score_version=""
profile=""
set_baseline="false"
result_list="$(mktemp "${TMPDIR:-/tmp}/nerdbench-calibrate.XXXXXX")"
trap 'rm -f "$result_list"' EXIT INT HUP TERM

add_result() {
  if [ -z "$1" ]; then
    echo "empty result path" >&2
    exit 1
  fi
  printf '%s\n' "$1" >>"$result_list"
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --score-version)
      need_value "$@"
      score_version="$2"
      shift 2
      ;;
    --result)
      need_value "$@"
      add_result "$2"
      shift 2
      ;;
    --profile)
      need_value "$@"
      profile="$2"
      shift 2
      ;;
    --set-baseline)
      set_baseline="true"
      shift
      ;;
    --runs|--out-dir)
      echo "$1 is no longer supported; run NerdBench separately and pass result JSON with --result" >&2
      exit 1
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    --*)
      echo "unknown argument: $1" >&2
      usage
      exit 1
      ;;
    *)
      add_result "$1"
      shift
      ;;
  esac
done

if [ -z "$score_version" ]; then
  echo "--score-version is required" >&2
  usage
  exit 1
fi

if [ ! -s "$result_list" ]; then
  echo "at least one result JSON file is required" >&2
  usage
  exit 1
fi

repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
candidate="$repo_root/calibration/baselines/$score_version.json"
baseline="$repo_root/internal/results/baselines/$score_version.json"

mkdir -p "$repo_root/calibration/baselines" "$repo_root/internal/results/baselines"

python3 - "$score_version" "$profile" "$candidate" "$result_list" <<'PY'
import json
import statistics
import sys
from pathlib import Path

score_version, expected_profile, candidate, result_list_path = sys.argv[1:]

with open(result_list_path, "r", encoding="utf-8") as f:
    result_files = [line.rstrip("\n") for line in f if line.rstrip("\n")]

docs = []
for path in result_files:
    p = Path(path)
    if not p.is_file():
        raise SystemExit(f"{path}: result file does not exist")
    with p.open("r", encoding="utf-8") as f:
        try:
            docs.append((path, json.load(f)))
        except json.JSONDecodeError as e:
            raise SystemExit(f"{path}: invalid JSON: {e}") from e

if not docs:
    raise SystemExit("no result files")

first_path, first = docs[0]
profile = first.get("profile")
if not profile:
    raise SystemExit(f"{first_path}: profile is required")
if expected_profile and profile != expected_profile:
    raise SystemExit(f"{first_path}: profile mismatch: got {profile!r}, expected {expected_profile!r}")

values = {}
expected_keys = None
for path, doc in docs:
    if doc.get("profile") != profile:
        raise SystemExit(f"{path}: profile mismatch: got {doc.get('profile')!r}, expected {profile!r}")
    if doc.get("status") != "ok":
        raise SystemExit(f"{path}: run status is not ok")

    run_keys = set()
    for b in doc.get("benchmarks", []):
        if b.get("mode") != "single":
            continue
        if b.get("status") != "ok":
            raise SystemExit(f"{path}: benchmark failed: {b.get('name')} {b.get('mode')}")
        metric = b.get("metric") or {}
        key = (b.get("name"), b.get("mode"), metric.get("name"), metric.get("unit"))
        if any(part in ("", None) for part in key):
            raise SystemExit(f"{path}: benchmark has incomplete metric identity")
        value = float(metric.get("value", 0))
        if value <= 0:
            raise SystemExit(f"{path}: benchmark has non-positive metric value: {b.get('name')} {b.get('mode')}")
        if key in run_keys:
            raise SystemExit(f"{path}: duplicate benchmark metric: {key}")
        run_keys.add(key)
        values.setdefault(key, []).append(value)

    if not run_keys:
        raise SystemExit(f"{path}: result has no single-core benchmark metrics")
    if expected_keys is None:
        expected_keys = run_keys
    elif run_keys != expected_keys:
        missing = sorted(expected_keys - run_keys)
        extra = sorted(run_keys - expected_keys)
        detail = []
        if missing:
            detail.append(f"missing {missing}")
        if extra:
            detail.append(f"extra {extra}")
        raise SystemExit(f"{path}: benchmark metric set mismatch: {'; '.join(detail)}")

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
