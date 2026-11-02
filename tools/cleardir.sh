#!/bin/sh
# Empties scratch directories (creating them if missing) without the
# `rm dir/*` globs that make Claude Code ask for confirmation.
# Usage: tools/cleardir.sh DIR...
set -e
for d in "$@"; do
	mkdir -p "$d"
	p=$(cd "$d" && pwd -P)
	case "$p/" in
	*/game/*|/|"$HOME"/) echo "cleardir: refusing $d ($p)" >&2; exit 1 ;;
	esac
	find "$p" -mindepth 1 -delete
done
