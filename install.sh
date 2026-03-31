#!/usr/bin/env sh
# kaal install script
# Usage: curl -fsSL https://raw.githubusercontent.com/mouhamedsylla/kaal/main/install.sh | sh
#
# Supports: macOS (arm64, amd64), Linux (arm64, amd64)
# Requirements: curl or wget, tar

set -e

REPO="mouhamedsylla/kaal"
BINARY="kaal"
INSTALL_DIR="${KAAL_INSTALL_DIR:-}"

# ──────────────── helpers ────────────────────────────────────────────────────

info()    { printf "  \033[34m→\033[0m  %s\n" "$1"; }
success() { printf "  \033[32m✓\033[0m  %s\n" "$1"; }
warn()    { printf "  \033[33m⚠\033[0m  %s\n" "$1"; }
error()   { printf "  \033[31m✗\033[0m  %s\n" "$1" >&2; exit 1; }

need_cmd() {
  if ! command -v "$1" > /dev/null 2>&1; then
    error "Required command '$1' not found. Please install it and try again."
  fi
}

# ──────────────── detect platform ────────────────────────────────────────────

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$OS" in
  linux)  OS="linux" ;;
  darwin) OS="darwin" ;;
  *)      error "Unsupported OS: $OS. Please build from source: https://github.com/$REPO" ;;
esac

case "$ARCH" in
  x86_64 | amd64)  ARCH="amd64" ;;
  aarch64 | arm64) ARCH="arm64" ;;
  *)               error "Unsupported architecture: $ARCH. Please build from source." ;;
esac

PLATFORM="${OS}_${ARCH}"

# ──────────────── choose install directory ────────────────────────────────────

if [ -z "$INSTALL_DIR" ]; then
  if [ -d "$HOME/bin" ] && echo "$PATH" | grep -q "$HOME/bin"; then
    INSTALL_DIR="$HOME/bin"
  elif [ -d "/usr/local/bin" ] && [ -w "/usr/local/bin" ]; then
    INSTALL_DIR="/usr/local/bin"
  elif [ -d "$HOME/.local/bin" ]; then
    INSTALL_DIR="$HOME/.local/bin"
  else
    INSTALL_DIR="$HOME/.local/bin"
    mkdir -p "$INSTALL_DIR"
    warn "Created $INSTALL_DIR — make sure it is in your PATH:"
    warn "  export PATH=\"\$HOME/.local/bin:\$PATH\""
  fi
fi

# ──────────────── fetch latest version ───────────────────────────────────────

printf "\n  \033[1mkaal installer\033[0m\n\n"

need_cmd curl

info "Detecting latest release..."
LATEST=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": "\(.*\)".*/\1/')

if [ -z "$LATEST" ]; then
  error "Could not determine latest version. Check https://github.com/$REPO/releases"
fi

info "Latest version: $LATEST"
info "Platform: $PLATFORM"
info "Install dir: $INSTALL_DIR"

# ──────────────── download and install ───────────────────────────────────────

TARBALL="${BINARY}_${LATEST#v}_${PLATFORM}.tar.gz"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${LATEST}/${TARBALL}"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

info "Downloading $TARBALL..."
curl -fsSL "$DOWNLOAD_URL" -o "$TMP_DIR/$TARBALL" || \
  error "Download failed. Check https://github.com/$REPO/releases/$LATEST"

info "Extracting..."
tar -xzf "$TMP_DIR/$TARBALL" -C "$TMP_DIR"

if [ ! -f "$TMP_DIR/$BINARY" ]; then
  error "Binary '$BINARY' not found in archive. The release may be malformed."
fi

chmod +x "$TMP_DIR/$BINARY"
mv "$TMP_DIR/$BINARY" "$INSTALL_DIR/$BINARY"

# ──────────────── verify installation ────────────────────────────────────────

if ! command -v kaal > /dev/null 2>&1; then
  warn "kaal installed to $INSTALL_DIR but is not in PATH."
  warn "Add this to your shell profile (~/.zshrc, ~/.bashrc, etc.):"
  warn "  export PATH=\"$INSTALL_DIR:\$PATH\""
  warn "Then restart your shell or run: source ~/.zshrc"
else
  INSTALLED_VERSION="$(kaal version 2>/dev/null || echo 'unknown')"
  success "kaal $LATEST installed successfully!"
  success "Version: $INSTALLED_VERSION"
fi

printf "\n  Next steps:\n"
printf "    kaal --help\n"
printf "    kaal init my-project\n\n"
