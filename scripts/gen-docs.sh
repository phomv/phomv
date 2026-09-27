#!/bin/sh
# Generates shell completions and man pages into ./completions and
# ./manpages for release archives and the Homebrew formula.
set -e
rm -rf completions manpages
mkdir -p completions manpages
bin=$(mktemp -d)/phomv
go build -o "$bin" ./cmd/phomv
for sh in bash zsh fish; do
	"$bin" completion "$sh" > "completions/phomv.$sh"
done
"$bin" completion powershell > completions/phomv.ps1
"$bin" man --dir manpages
rm -rf "$(dirname "$bin")"
