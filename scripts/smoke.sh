#!/usr/bin/env sh
set -eu

repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
go test ./...
go run "$repo_root/cmd/nerdbench" run --profile smoke --format json --progress none -o "$repo_root/smoke-result.json" >/dev/null
python3 -m json.tool "$repo_root/smoke-result.json" >/dev/null
rm -f "$repo_root/smoke-result.json"
