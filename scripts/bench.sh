#!/usr/bin/env sh
set -eu

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"
repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"

if command -v go >/dev/null 2>&1 && [ -f "$repo_root/go.mod" ] && [ ! -x "$repo_root/bin/nerdbench-${os}-${arch}" ]; then
  exec go run "$repo_root/cmd/nerdbench" run "$@"
fi

case "$os" in
  linux) os="linux" ;;
  *) echo "unsupported OS: $os" >&2; exit 1 ;;
esac

case "$arch" in
  x86_64|amd64) arch="amd64" ;;
  aarch64|arm64) arch="arm64" ;;
  *) echo "unsupported arch: $arch" >&2; exit 1 ;;
esac

asset="nerdbench-${os}-${arch}"

if [ -x "$repo_root/bin/$asset" ]; then
  exec "$repo_root/bin/$asset" run "$@"
fi

base_url="${NERDBENCH_BASE_URL:-}"
if [ -z "$base_url" ]; then
  echo "NERDBENCH_BASE_URL is not set and no local binary/go toolchain is available" >&2
  exit 1
fi

tmp="${TMPDIR:-/tmp}/nerdbench.$$"
mkdir -p "$tmp"
trap 'rm -rf "$tmp"' EXIT INT TERM

download() {
  url="$1"
  out="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$out"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$out" "$url"
  else
    echo "curl or wget is required to download NerdBench" >&2
    exit 1
  fi
}

download "$base_url/$asset" "$tmp/$asset"
download "$base_url/SHA256SUMS" "$tmp/SHA256SUMS"

if command -v sha256sum >/dev/null 2>&1; then
  (cd "$tmp" && grep "  $asset\$" SHA256SUMS | sha256sum -c -)
elif command -v shasum >/dev/null 2>&1; then
  expected="$(grep "  $asset\$" "$tmp/SHA256SUMS" | awk '{print $1}')"
  actual="$(shasum -a 256 "$tmp/$asset" | awk '{print $1}')"
  [ "$expected" = "$actual" ] || { echo "checksum verification failed" >&2; exit 1; }
else
  echo "warning: no sha256 tool found; skipping checksum verification" >&2
fi

chmod +x "$tmp/$asset"
exec "$tmp/$asset" run "$@"
