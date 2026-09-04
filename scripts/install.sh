#!/bin/sh
set -eu

repo="${WEBHOOK_REPO:-xtulnx/webhook}"
install_dir="${WEBHOOK_INSTALL_DIR:-/usr/local/bin}"
version="${WEBHOOK_VERSION:-latest}"
binary_name="${WEBHOOK_BINARY_NAME:-webhook}"

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "error: required command not found: $1" >&2
    exit 1
  fi
}

detect_os() {
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$os" in
    darwin|freebsd|linux|openbsd) printf '%s' "$os" ;;
    *) echo "error: unsupported OS: $os" >&2; exit 1 ;;
  esac
}

detect_arch() {
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64) printf 'amd64' ;;
    aarch64|arm64) printf 'arm64' ;;
    i386|i686) printf '386' ;;
    armv6l|armv7l|arm) printf 'arm' ;;
    *) echo "error: unsupported architecture: $arch" >&2; exit 1 ;;
  esac
}

download() {
  url="$1"
  output="$2"

  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$output"
  elif command -v wget >/dev/null 2>&1; then
    wget -q "$url" -O "$output"
  else
    echo "error: curl or wget is required" >&2
    exit 1
  fi
}

verify_checksum() {
  checksums="$1"
  archive="$2"
  asset="$3"

  expected="$(grep "  $asset\$" "$checksums" | awk '{print $1}')"
  if [ -z "$expected" ]; then
    echo "error: checksum for $asset not found" >&2
    exit 1
  fi

  if command -v sha256sum >/dev/null 2>&1; then
    actual="$(sha256sum "$archive" | awk '{print $1}')"
  elif command -v shasum >/dev/null 2>&1; then
    actual="$(shasum -a 256 "$archive" | awk '{print $1}')"
  elif command -v sha256 >/dev/null 2>&1; then
    actual="$(sha256 -q "$archive")"
  else
    echo "warning: no SHA256 command found; skipping checksum verification" >&2
    return
  fi

  if [ "$actual" != "$expected" ]; then
    echo "error: checksum mismatch for $asset" >&2
    exit 1
  fi
}

install_binary() {
  src="$1"
  dst="$2"

  mkdir -p "$install_dir" 2>/dev/null || true
  if [ -w "$install_dir" ]; then
    install -m 0755 "$src" "$dst"
  elif command -v sudo >/dev/null 2>&1; then
    sudo mkdir -p "$install_dir"
    sudo install -m 0755 "$src" "$dst"
  else
    echo "error: $install_dir is not writable and sudo is not available" >&2
    exit 1
  fi
}

need uname
need tar

os="$(detect_os)"
arch="$(detect_arch)"
asset="webhook-$os-$arch.tar.gz"

if [ "$version" = "latest" ]; then
  url="https://github.com/$repo/releases/latest/download/$asset"
  checksums_url="https://github.com/$repo/releases/latest/download/checksums.txt"
else
  url="https://github.com/$repo/releases/download/$version/$asset"
  checksums_url="https://github.com/$repo/releases/download/$version/checksums.txt"
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT INT TERM

archive="$tmp/$asset"
echo "Downloading $repo $version for $os/$arch..."
download "$url" "$archive"
download "$checksums_url" "$tmp/checksums.txt"
verify_checksum "$tmp/checksums.txt" "$archive" "$asset"

tar -xzf "$archive" -C "$tmp"
install_binary "$tmp/webhook-$os-$arch/webhook" "$install_dir/$binary_name"

echo "Installed $binary_name to $install_dir"
"$install_dir/$binary_name" -version
