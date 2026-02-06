#!/usr/bin/env bash
# Generate Go code from proto files. Run from repo root.
set -e
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT/proto"
docker run --rm -v "$ROOT:/workspace" -w /workspace/proto bufbuild/buf:latest generate
echo "Generated code in $ROOT/gen"
