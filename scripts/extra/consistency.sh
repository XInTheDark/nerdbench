#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat >&2 <<'EOF'
usage: scripts/extra/consistency.sh --iters N [bench args...]

Run scripts/bench.sh repeatedly with the supplied bench args, then print
consistency stats and write a PNG graph.

The helper appends these arguments to each bench run:
  --format json --progress none -o <temporary result file>

Environment:
  NERDBENCH_BENCH_SH          bench runner path; defaults to scripts/bench.sh
  NERDBENCH_CONSISTENCY_PNG  output PNG path; defaults to nerdbench-consistency-<timestamp>.png
EOF
}

iters=""
bench_args=()

while [ "$#" -gt 0 ]; do
  case "$1" in
    --iters)
      if [ "$#" -lt 2 ]; then
        echo "--iters requires a value" >&2
        exit 1
      fi
      iters="$2"
      shift 2
      ;;
    --iters=*)
      iters="${1#--iters=}"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    --)
      shift
      bench_args+=("$@")
      break
      ;;
    *)
      bench_args+=("$1")
      shift
      ;;
  esac
done

case "$iters" in
  ''|*[!0-9]*)
    echo "--iters must be a positive integer" >&2
    usage
    exit 1
    ;;
esac

if [ "$iters" -lt 1 ]; then
  echo "--iters must be > 0" >&2
  exit 1
fi

repo_root="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
bench_sh="${NERDBENCH_BENCH_SH:-$repo_root/scripts/bench.sh}"
out_png="${NERDBENCH_CONSISTENCY_PNG:-$PWD/nerdbench-consistency-$(date +%Y%m%d-%H%M%S).png}"

if [ ! -x "$bench_sh" ]; then
  echo "bench runner is not executable: $bench_sh" >&2
  exit 1
fi

tmp="$(mktemp -d "${TMPDIR:-/tmp}/nerdbench-consistency.XXXXXX")"
trap 'rm -rf "$tmp"' EXIT INT HUP TERM

result_files=()

for ((i = 1; i <= iters; i++)); do
  run_file="$tmp/run-$(printf '%03d' "$i").json"
  stdout_file="$tmp/run-$(printf '%03d' "$i").stdout"
  echo "run $i/$iters" >&2
  if ! "$bench_sh" "${bench_args[@]}" --format json --progress none -o "$run_file" >"$stdout_file"; then
    echo "bench run $i failed" >&2
    if [ -s "$stdout_file" ]; then
      cat "$stdout_file" >&2
    fi
    exit 1
  fi
  if [ ! -s "$run_file" ] && [ -s "$stdout_file" ]; then
    cp "$stdout_file" "$run_file"
  fi
  if [ ! -s "$run_file" ]; then
    echo "bench run $i did not produce JSON output" >&2
    exit 1
  fi
  result_files+=("$run_file")
done

python3 - "$out_png" "${result_files[@]}" <<'PY'
import json
import math
import statistics
import struct
import sys
import zlib
from pathlib import Path

out_png = Path(sys.argv[1])
run_files = [Path(path) for path in sys.argv[2:]]

docs = []
for path in run_files:
    with path.open("r", encoding="utf-8") as f:
        docs.append(json.load(f))

def mean(values):
    return statistics.fmean(values) if values else 0.0

def stdev(values):
    return statistics.stdev(values) if len(values) > 1 else 0.0

def cv(values):
    avg = mean(values)
    return (stdev(values) / abs(avg) * 100.0) if avg else 0.0

def row_stats(values):
    return {
        "mean": mean(values),
        "stdev": stdev(values),
        "cv": cv(values),
        "min": min(values) if values else 0.0,
        "max": max(values) if values else 0.0,
    }

def fmt(value):
    if abs(value) >= 100:
        return f"{value:.2f}"
    if abs(value) >= 1:
        return f"{value:.4f}"
    return f"{value:.6f}"

for idx, doc in enumerate(docs, start=1):
    if doc.get("status") != "ok":
        raise SystemExit(f"run {idx}: status is not ok")

single_scores = [float(doc.get("scores", {}).get("single") or 0) for doc in docs]
multi_scores = [float(doc.get("scores", {}).get("multi") or 0) for doc in docs]

metrics = {}
score_series = {}
for doc in docs:
    seen = set()
    for bench in doc.get("benchmarks", []):
        if bench.get("status") != "ok":
            raise SystemExit(f"{bench.get('name')} {bench.get('mode')}: status is not ok")
        metric = bench.get("metric") or {}
        key = (
            str(bench.get("name")),
            str(bench.get("mode")),
            str(metric.get("name")),
            str(metric.get("unit")),
        )
        if any(part in ("", "None") for part in key):
            raise SystemExit("benchmark has incomplete metric identity")
        if key in seen:
            raise SystemExit(f"duplicate benchmark metric: {key}")
        seen.add(key)
        metrics.setdefault(key, []).append(float(metric.get("value") or 0))
        score_series.setdefault(key, []).append(float(bench.get("score") or 0))

expected_len = len(docs)
for key, values in metrics.items():
    if len(values) != expected_len:
        raise SystemExit(f"{key}: missing values in one or more runs")

print(f"NerdBench consistency over {len(docs)} runs")
print()
print("Overall scores")
print("name       mean        stdev       cv%      min         max")
for name, values in (("single", single_scores), ("multi", multi_scores)):
    stats = row_stats(values)
    print(
        f"{name:<10} {fmt(stats['mean']):>10} {fmt(stats['stdev']):>10} "
        f"{stats['cv']:>8.3f} {fmt(stats['min']):>10} {fmt(stats['max']):>10}"
    )

print()
print("Benchmark metrics")
print("benchmark/mode/metric                         mean        stdev       cv%      min         max")
for key in sorted(metrics):
    label = f"{key[0]}/{key[1]}/{key[2]}"
    stats = row_stats(metrics[key])
    print(
        f"{label:<42} {fmt(stats['mean']):>10} {fmt(stats['stdev']):>10} "
        f"{stats['cv']:>8.3f} {fmt(stats['min']):>10} {fmt(stats['max']):>10}"
    )

def write_png(path, width, height, pixels):
    raw = bytearray()
    for y in range(height):
        raw.append(0)
        row = pixels[y]
        for r, g, b in row:
            raw.extend((r, g, b))
    def chunk(kind, data):
        return (
            struct.pack(">I", len(data))
            + kind
            + data
            + struct.pack(">I", zlib.crc32(kind + data) & 0xFFFFFFFF)
        )
    data = (
        b"\x89PNG\r\n\x1a\n"
        + chunk(b"IHDR", struct.pack(">IIBBBBB", width, height, 8, 2, 0, 0, 0))
        + chunk(b"IDAT", zlib.compress(bytes(raw), 9))
        + chunk(b"IEND", b"")
    )
    path.write_bytes(data)

def fallback_png(path):
    width, height = 1000, 600
    pixels = [[(255, 255, 255) for _ in range(width)] for _ in range(height)]

    def line(x0, y0, x1, y1, color):
        dx = abs(x1 - x0)
        dy = -abs(y1 - y0)
        sx = 1 if x0 < x1 else -1
        sy = 1 if y0 < y1 else -1
        err = dx + dy
        while True:
            if 0 <= x0 < width and 0 <= y0 < height:
                pixels[y0][x0] = color
            if x0 == x1 and y0 == y1:
                break
            e2 = 2 * err
            if e2 >= dy:
                err += dy
                x0 += sx
            if e2 <= dx:
                err += dx
                y0 += sy

    def rect(x0, y0, x1, y1, color):
        for y in range(max(0, y0), min(height, y1 + 1)):
            for x in range(max(0, x0), min(width, x1 + 1)):
                pixels[y][x] = color

    left, right, top, bottom = 70, 970, 50, 360
    axis = (70, 70, 70)
    line(left, bottom, right, bottom, axis)
    line(left, top, left, bottom, axis)

    series = [
        ("single", single_scores, (36, 99, 235)),
        ("multi", multi_scores, (220, 38, 38)),
    ]
    all_scores = [v for _, values, _ in series for v in values if v > 0]
    score_min = min(all_scores) if all_scores else 0.0
    score_max = max(all_scores) if all_scores else 1.0
    if math.isclose(score_min, score_max):
        score_min *= 0.95
        score_max *= 1.05
    span = score_max - score_min or 1.0
    x_span = right - left
    y_span = bottom - top

    for _, values, color in series:
        points = []
        for idx, value in enumerate(values):
            x = left + int(x_span * (idx / max(1, len(values) - 1)))
            y = bottom - int(y_span * ((value - score_min) / span))
            points.append((x, y))
            rect(x - 3, y - 3, x + 3, y + 3, color)
        for a, b in zip(points, points[1:]):
            line(a[0], a[1], b[0], b[1], color)

    bar_top, bar_bottom = 430, 560
    cv_items = []
    for key, values in metrics.items():
        cv_items.append((f"{key[0]}/{key[1]}", cv(values)))
    cv_items.sort(key=lambda item: item[1], reverse=True)
    cv_items = cv_items[:20]
    max_cv = max([item[1] for item in cv_items] + [1.0])
    bar_width = max(8, int((right - left) / max(1, len(cv_items)) * 0.65))
    step = (right - left) / max(1, len(cv_items))
    line(left, bar_bottom, right, bar_bottom, axis)
    for idx, (_, value) in enumerate(cv_items):
        x = int(left + idx * step + (step - bar_width) / 2)
        h = int((bar_bottom - bar_top) * (value / max_cv))
        rect(x, bar_bottom - h, x + bar_width, bar_bottom, (16, 185, 129))

    write_png(path, width, height, pixels)

try:
    import matplotlib.pyplot as plt

    runs = list(range(1, len(docs) + 1))
    fig, axes = plt.subplots(2, 1, figsize=(11, 7), constrained_layout=True)
    axes[0].plot(runs, single_scores, marker="o", label="single")
    axes[0].plot(runs, multi_scores, marker="o", label="multi")
    axes[0].set_title("Overall score consistency")
    axes[0].set_xlabel("run")
    axes[0].set_ylabel("score")
    axes[0].grid(True, alpha=0.25)
    axes[0].legend()

    cv_items = [(f"{key[0]}/{key[1]}", cv(values)) for key, values in metrics.items()]
    cv_items.sort(key=lambda item: item[1], reverse=True)
    cv_items = cv_items[:20]
    labels = [item[0] for item in cv_items]
    values = [item[1] for item in cv_items]
    axes[1].bar(labels, values)
    axes[1].set_title("Metric coefficient of variation")
    axes[1].set_xlabel("benchmark")
    axes[1].set_ylabel("cv%")
    axes[1].tick_params(axis="x", labelrotation=70)
    axes[1].grid(axis="y", alpha=0.25)

    out_png.parent.mkdir(parents=True, exist_ok=True)
    fig.savefig(out_png, dpi=150)
except Exception:
    out_png.parent.mkdir(parents=True, exist_ok=True)
    fallback_png(out_png)

print()
print(f"graph: {out_png}")
PY
