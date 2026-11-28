#!/usr/bin/env bash
set -euo pipefail

outdir=${1:?usage: build-cli-release.sh OUTPUT_DIRECTORY}
mkdir -p "$outdir"
assets=()

for os in linux darwin windows; do
  for arch in amd64 arm64; do
    name="scrapemate-${os}-${arch}"
    if [[ "$os" == windows ]]; then
      name+=".exe"
    fi
    CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath -ldflags='-s -w' -o "$outdir/$name" ./cmd/scrapemate
    assets+=("$name")
  done
done

(cd "$outdir" && sha256sum "${assets[@]}" > SHA256SUMS)
