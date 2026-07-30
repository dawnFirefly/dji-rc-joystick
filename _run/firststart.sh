#!/usr/bin/env bash

set -Eeuo pipefail

run_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root_path="$(cd "$run_dir/.." && pwd)"
gometagen="github.com/amazing-generators/gometagen/cmd/gometagen@latest"

cd "$root_path"

mkdir -p target tmp

go install fyne.io/tools/cmd/fyne@latest
go install github.com/fyne-io/fyne-cross@v1.6.2

go install github.com/amazing-generators/gometagen/cmd/gometagen@latest
go install github.com/amazing-generators/goconfgen/cmd/goconfgen@latest
go install github.com/amazing-generators/godepsgen/cmd/godepsgen@latest

go run "$gometagen" git add-commit-hook -source "$root_path"
go run "$gometagen" git add-push-hook -source "$root_path"

go work sync
go generate .

go list -m -f '{{if .Main}}{{.Dir}}{{end}}' all | while read -r dir; do
  [ -n "$dir" ] || continue
  echo "  -> $dir"
  go -C "$dir" mod tidy
done

go generate .
