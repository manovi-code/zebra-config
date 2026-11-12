#!/bin/bash
# Build Zebra Config.app and Zebra-Config-macOS.zip on macOS.
# ditto keeps the executable bit, which a normal zip from Linux often drops.
# Usage: build-app.sh <universal-binary> <version> <output-dir>
set -euo pipefail

if [[ $# -ne 3 ]]; then
	echo "usage: $0 <universal-binary> <version> <output-dir>" >&2
	exit 1
fi

bin=$1
version=$2
out=$3

if [[ ! -f "$bin" ]]; then
	echo "binary not found: $bin" >&2
	exit 1
fi
if [[ ! "$version" =~ ^[0-9]+(\.[0-9]+){1,2}$ ]]; then
	echo "version must look like 0.1.1, got: $version" >&2
	exit 1
fi
if ! command -v ditto >/dev/null; then
	echo "ditto is required. Run this on macOS." >&2
	exit 1
fi

script_dir=$(cd "$(dirname "$0")" && pwd)
app="$out/Zebra Config.app"
rm -rf "$app"
mkdir -p "$app/Contents/MacOS"
cp "$bin" "$app/Contents/MacOS/zebra-config"
chmod 755 "$app/Contents/MacOS/zebra-config"
sed "s/__VERSION__/${version}/g" "$script_dir/Info.plist" > "$app/Contents/Info.plist"

# Ad-hoc signature. It does not skip Gatekeeper on a downloaded app.
# It does make the bundle a real signed app instead of a loose binary.
if command -v codesign >/dev/null; then
	codesign --force --deep --sign - "$app"
	codesign --verify --verbose=2 "$app"
fi

zip="$out/Zebra-Config-macOS.zip"
rm -f "$zip"
ditto -c -k --keepParent "$app" "$zip"

check=$(mktemp -d)
trap 'rm -rf "$check"' EXIT
ditto -x -k "$zip" "$check"
inner="$check/Zebra Config.app/Contents/MacOS/zebra-config"
if [[ ! -x "$inner" ]]; then
	echo "the zip lost the executable bit on $inner" >&2
	exit 1
fi
echo "wrote $zip"
