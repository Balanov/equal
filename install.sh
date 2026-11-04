#!/bin/sh
# Install EQUAL from the latest GitHub release with a SHA-256 checksum check.
# Usage: curl -fsSL https://raw.githubusercontent.com/Balanov/equal/main/install.sh | sh
# Optional: EQUAL_VERSION=0.1.0 EQUAL_INSTALL_DIR=/usr/local/bin sh install.sh
set -eu

repo="Balanov/equal"
if [ "${EQUAL_VERSION:-}" = "" ]; then
  url=$(curl -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/$repo/releases/latest")
  version="${url##*/}"
else
  version="$EQUAL_VERSION"
fi
version="${version#v}"
[ -n "$version" ] || { echo "equal: could not resolve release version" >&2; exit 1; }

os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)
case "$os" in
  linux|darwin) ;;
  *) echo "equal: unsupported OS '$os' (use install.ps1 on Windows)" >&2; exit 1 ;;
esac
case "$arch" in
  x86_64|amd64) arch="amd64" ;;
  aarch64|arm64) arch="arm64" ;;
  *) echo "equal: unsupported architecture '$arch'" >&2; exit 1 ;;
esac

asset="equal-${os}-${arch}"
base="https://github.com/$repo/releases/download/v$version"
install_dir="${EQUAL_INSTALL_DIR:-$HOME/.local/bin}"
if command -v sha256sum >/dev/null 2>&1; then
  sha256() { sha256sum "$1" | awk '{print $1}'; }
elif command -v shasum >/dev/null 2>&1; then
  sha256() { shasum -a 256 "$1" | awk '{print $1}'; }
else
  echo "equal: sha256sum or shasum is required" >&2; exit 1
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
curl -fsSL --retry 3 -o "$tmp/$asset" "$base/$asset"
curl -fsSL --retry 3 -o "$tmp/checksums.txt" "$base/checksums.txt"
expected=""
while read -r sum name; do
  name="${name#\*}"
  [ "$name" = "$asset" ] && expected="$sum"
done < "$tmp/checksums.txt"
[ -n "$expected" ] || { echo "equal: no checksum for $asset" >&2; exit 1; }
actual=$(sha256 "$tmp/$asset")
[ "$actual" = "$expected" ] || { echo "equal: checksum mismatch for $asset" >&2; exit 1; }

mkdir -p "$install_dir"
target_tmp="$install_dir/.equal-install-$$"
cp "$tmp/$asset" "$target_tmp"
chmod 0755 "$target_tmp"
mv -f "$target_tmp" "$install_dir/equal"
installed=$("$install_dir/equal" --version)
echo "installed: $installed -> $install_dir/equal"
case ":$PATH:" in
  *":$install_dir:"*) ;;
  *) echo "note: $install_dir is not in PATH" ;;
esac
