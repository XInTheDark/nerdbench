#!/usr/bin/env sh
set -eu

script_url="${NERDBENCH_SCRIPT_URL:-https://raw.githubusercontent.com/XInTheDark/nerdbench/main/scripts/bench.sh}"

if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$script_url" | sh -s -- "$@"
elif command -v wget >/dev/null 2>&1; then
  wget -qO- "$script_url" | sh -s -- "$@"
else
  echo "curl or wget is required to download NerdBench" >&2
  exit 1
fi
