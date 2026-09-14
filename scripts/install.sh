#!/usr/bin/env bash
set -euo pipefail

REPO="AnshGajera/CTX"
BIN_DIR="${HOME}/.ctx/bin"
VERSION="${1:-latest}"

detect_os() {
  case "$(uname -s)" in
    Linux*) echo "linux" ;;
    Darwin*) echo "darwin" ;;
    *) echo "unsupported OS: $(uname -s)" >&2; exit 1 ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo "amd64" ;;
    arm64|aarch64) echo "arm64" ;;
    *) echo "unsupported arch: $(uname -m)" >&2; exit 1 ;;
  esac
}

OS=$(detect_os)
ARCH=$(detect_arch)

if [ "$VERSION" = "latest" ]; then
  VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)
fi
echo "Installing ctx ${VERSION} (${OS}/${ARCH})..."

TARBALL="ctx_${VERSION#v}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${TARBALL}"

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
cd "$TMP"
curl -fsSL -o "$TARBALL" "$URL"
curl -fsSL -o checksums.txt "https://github.com/${REPO}/releases/download/${VERSION}/checksums.txt"

if command -v sha256sum >/dev/null; then
  grep "$TARBALL" checksums.txt | sha256sum -c -
elif command -v shasum >/dev/null; then
  grep "$TARBALL" checksums.txt | shasum -a 256 -c -
fi

mkdir -p "$BIN_DIR"
tar -xzf "$TARBALL" -C "$BIN_DIR"
chmod +x "$BIN_DIR/ctx"

# PATH setup
SHELL_NAME=$(basename "${SHELL:-bash}")
case "$SHELL_NAME" in
  zsh) PROFILE="$HOME/.zshrc" ;;
  fish) PROFILE="$HOME/.config/fish/config.fish" ;;
  *) PROFILE="$HOME/.bashrc" ;;
esac

if [ "$SHELL_NAME" = "fish" ]; then
  grep -q "$BIN_DIR" "$PROFILE" 2>/dev/null || echo "set -gx PATH \$PATH $BIN_DIR" >> "$PROFILE"
else
  grep -q "$BIN_DIR" "$PROFILE" 2>/dev/null || echo "export PATH=\"\$PATH:$BIN_DIR\"" >> "$PROFILE"
fi

# default config
mkdir -p "$HOME/.config/ctx"
if [ ! -f "$HOME/.config/ctx/config.toml" ]; then
  cat > "$HOME/.config/ctx/config.toml" <<'EOF'
[core]
api_url = "https://api.ctx.dev"
auto_sync = true
watch_mode = true
ml_url = "http://localhost:8001"

[extraction]
architecture = true
api_endpoints = true
database_schema = true
dependencies = true
business_rules = true
env_vars = true

[privacy]
exclude_patterns = ["*.pem", "*.key", "secrets/*"]
redact_values = true
hash_identifiers = false

[sync]
interval_seconds = 300
on_git_commit = true
on_file_save = false
EOF
fi

echo "Installed to $BIN_DIR/ctx"
echo "Restart your shell or run: export PATH=\"\$PATH:$BIN_DIR\""
echo "Next: ctx init && ctx extract && ctx status"
