#!/bin/sh
# Installer for symvibe — the Symaira Vibe Coding Baukasten.
# Usage: curl -fsSL https://raw.githubusercontent.com/danieljustus/symaira-vibecoder/main/scripts/install.sh | sh
set -eu

REPO="danieljustus/symaira-vibecoder"
PREFIX="${SYMVIBE_INSTALL_PREFIX:-}"

if [ -z "$PREFIX" ]; then
	if [ -w "/usr/local/bin" ]; then
		PREFIX="/usr/local/bin"
	else
		PREFIX="${HOME}/.local/bin"
	fi
fi

command -v curl >/dev/null 2&1 || { echo "curl is required"; exit 1; }
command -v uname >/dev/null 2&1 || { echo "uname is required"; exit 1; }

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
	x86_64) ARCH="amd64" ;;
	arm64|aarch64) ARCH="arm64" ;;
	*) echo "unsupported architecture: $ARCH"; exit 1 ;;
esac
case "$OS" in
	darwin|linux) ;;
	*) echo "unsupported OS: $OS"; exit 1 ;;
esac

# Fetch the latest release tag from GitHub.
TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p')
if [ -z "$TAG" ]; then
	echo "could not determine latest release"
	exit 1
fi

ASSET="symvibe_${TAG}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${TAG}/${ASSET}"

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

echo "Downloading symvibe ${TAG} for ${OS}/${ARCH}..."
curl -fsSL "$URL" -o "${TMPDIR}/${ASSET}"

echo "Extracting..."
tar -xzf "${TMPDIR}/${ASSET}" -C "$TMPDIR"

mkdir -p "$PREFIX"
cp "${TMPDIR}/symvibe" "${PREFIX}/symvibe"
chmod +x "${PREFIX}/symvibe"

echo "Installed symvibe to ${PREFIX}/symvibe"
echo "Make sure ${PREFIX} is on your PATH."
