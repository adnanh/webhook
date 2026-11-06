#!/bin/sh
# Re-run to install/update. Configuration is supplied through WEBHOOK_* variables.
set -eu

repo="${WEBHOOK_REPO:-xtulnx/webhook}"
install_dir="${WEBHOOK_INSTALL_DIR:-/usr/local/bin}"
version="${WEBHOOK_VERSION:-latest}"
binary_name="${WEBHOOK_BINARY_NAME:-webhook}"

fail() { echo "error: $*" >&2; exit 1; }
need() { command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"; }

need uname
need tar
need awk
need sed
need install

# Keep user-controlled values out of paths and URL components.
printf '%s\n' "$repo" | LC_ALL=C awk '/^[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+$/ {ok=1} END {exit !(NR == 1 && ok)}' || fail 'WEBHOOK_REPO must be owner/repo'
case "$binary_name" in
  ''|[!a-zA-Z0-9]*|*[!a-zA-Z0-9._-]*) fail 'WEBHOOK_BINARY_NAME must be a filename starting with a letter or number' ;;
esac
case "$install_dir" in
  /*) ;;
  *) install_dir="$(pwd)/$install_dir" ;;
esac

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$os" in
  darwin|freebsd|linux|openbsd) ;;
  *) fail "unsupported OS: $os" ;;
esac
case "$(uname -m)" in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  i386|i486|i586|i686) arch=386 ;;
  armv6l|armv7l|arm) arch=arm ;;
  *) fail "unsupported architecture: $(uname -m)" ;;
esac
case "$os/$arch" in
  darwin/amd64|darwin/arm64|linux/386|linux/amd64|linux/arm|linux/arm64|freebsd/386|freebsd/amd64|freebsd/arm64|openbsd/386|openbsd/amd64|openbsd/arm64) ;;
  *) fail "no release asset for $os/$arch" ;;
esac

# Verification is mandatory and happens before changing the installation.
if command -v sha256sum >/dev/null 2>&1; then
  hash_tool=sha256sum
elif command -v shasum >/dev/null 2>&1; then
  hash_tool=shasum
elif command -v sha256 >/dev/null 2>&1; then
  hash_tool=sha256
else
  fail 'sha256sum, shasum or sha256 is required'
fi

download() {
  if command -v curl >/dev/null 2>&1; then
    curl --fail --silent --show-error --location --retry 3 --connect-timeout 15 --max-time 300 --output "$2" "$1"
  elif command -v wget >/dev/null 2>&1; then
    wget -q --tries=3 --timeout=30 -O "$2" "$1"
  else
    fail 'curl or wget is required'
  fi
}

use_sudo=false
stage=
tmp="$(mktemp -d)"
as_root() {
  if [ "$use_sudo" = true ]; then sudo "$@"; else "$@"; fi
}
cleanup() {
  [ -z "$stage" ] || as_root rm -f "$stage"
  rm -rf "$tmp"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

# Resolve latest once so the archive and checksums always come from the same tag.
if [ "$version" = latest ]; then
  download "https://api.github.com/repos/$repo/releases/latest" "$tmp/release.json"
  version="$(sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$tmp/release.json")"
else
  case "$version" in v*) ;; *) version="v$version" ;; esac
fi
printf '%s\n' "$version" | LC_ALL=C awk '/^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$/ {ok=1} END {exit !(NR == 1 && ok)}' || fail 'invalid release version (expected vX.Y.Z)'

asset="webhook-$os-$arch.tar.gz"
archive="$tmp/$asset"
base_url="https://github.com/$repo/releases/download/$version"
echo "Downloading $repo $version for $os/$arch..."
download "$base_url/$asset" "$archive"
download "$base_url/checksums.txt" "$tmp/checksums.txt"
expected="$(awk -v asset="$asset" '$2 == asset {print $1; count++} END {if (count != 1) exit 1}' "$tmp/checksums.txt")" || fail "expected exactly one checksum for $asset"
[ "${#expected}" -eq 64 ] || fail "invalid checksum for $asset"
case "$expected" in *[!0-9a-fA-F]*) fail "invalid checksum for $asset" ;; esac
case "$hash_tool" in
  sha256sum) actual="$(sha256sum "$archive" | awk '{print $1}')" ;;
  shasum) actual="$(shasum -a 256 "$archive" | awk '{print $1}')" ;;
  sha256) actual="$(sha256 -q "$archive")" ;;
esac
expected="$(printf '%s' "$expected" | tr '[:upper:]' '[:lower:]')"
[ "$actual" = "$expected" ] || fail "checksum mismatch for $asset"

# Extract only the expected binary; never install an archive-provided symlink.
member="webhook-$os-$arch/webhook"
tar -xzf "$archive" -C "$tmp" "$member"
source="$tmp/$member"
[ -f "$source" ] && [ ! -L "$source" ] || fail 'archive does not contain a regular webhook binary'
chmod 0755 "$source"
reported_version="$("$source" -version)" || fail 'downloaded binary cannot run on this machine'
[ "$reported_version" = "webhook version ${version#v}" ] || fail "downloaded binary reported an unexpected version: $reported_version"

if ! mkdir -p "$install_dir" 2>/dev/null || [ ! -w "$install_dir" ]; then
  need sudo
  use_sudo=true
  as_root mkdir -p "$install_dir"
fi
# Stage in the destination filesystem then rename: running Unix binaries can be updated.
[ ! -d "$install_dir/$binary_name" ] || fail "target is a directory: $install_dir/$binary_name"
stage="$(as_root mktemp "$install_dir/.${binary_name}.XXXXXX")"
as_root install -m 0755 "$source" "$stage"
as_root mv -f "$stage" "$install_dir/$binary_name"
stage=
echo "Installed $reported_version to $install_dir/$binary_name"
case ":$PATH:" in
  *":$install_dir:"*) ;;
  *) echo "Add $install_dir to PATH to use $binary_name directly." ;;
esac
echo 'Restart any running webhook service to use the installed version.'
