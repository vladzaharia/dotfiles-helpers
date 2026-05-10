#!/bin/bash
set -euo pipefail

REPO="vladzaharia/dotfiles-helpers"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
TOOLS=(agent-helper vault-helper sops-helper)

usage() {
    cat <<EOF
Usage: install.sh [tool|all|--help|--list]

Tools:
$(printf '  %s\n' "${TOOLS[@]}")
  all                Install every tool (default)

Flags:
  --help, -h         Show this help
  --list             List supported tools

Environment:
  INSTALL_DIR        Target directory (default: \$HOME/.local/bin)
EOF
}

case "${1:-all}" in
    -h|--help)
        usage
        exit 0
        ;;
    --list)
        printf '%s\n' "${TOOLS[@]}"
        exit 0
        ;;
esac

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
[[ "$ARCH" == "x86_64" ]] && ARCH="amd64"
[[ "$ARCH" == "aarch64" ]] && ARCH="arm64"

case "$OS" in
    darwin|linux) ;;
    *)
        echo "error: unsupported OS '$OS'" >&2
        exit 1
        ;;
esac

case "$ARCH" in
    amd64|arm64) ;;
    *)
        echo "error: unsupported arch '$ARCH'" >&2
        exit 1
        ;;
esac

VERSION=$(curl -sSf "https://api.github.com/repos/$REPO/releases/latest" | grep tag_name | cut -d'"' -f4 || true)
if [[ -z "$VERSION" ]]; then
    echo "error: could not resolve latest release (GitHub API rate-limit or network failure)" >&2
    echo "       browse https://github.com/$REPO/releases for manual download" >&2
    exit 1
fi

install_tool() {
    local tool="$1"
    local url="https://github.com/$REPO/releases/download/$VERSION/${tool}_${OS}_${ARCH}.tar.gz"
    curl -sSfL "$url" | tar xz -C "$INSTALL_DIR" "$tool"
    echo "Installed $tool $VERSION to $INSTALL_DIR"
}

mkdir -p "$INSTALL_DIR"

case "${1:-all}" in
    agent-helper)
        install_tool agent-helper
        ln -sf "$INSTALL_DIR/agent-helper" "$INSTALL_DIR/ag"
        ;;
    vault-helper)
        install_tool vault-helper
        for a in vh vlogin vssh vmosh votp vtoken vnv vdocker; do
            ln -sf "$INSTALL_DIR/vault-helper" "$INSTALL_DIR/$a"
        done
        ;;
    sops-helper)
        install_tool sops-helper
        ln -sf "$INSTALL_DIR/sops-helper" "$INSTALL_DIR/crypto"
        ;;
    all)
        for t in "${TOOLS[@]}"; do
            "$0" "$t"
        done
        ;;
    *)
        echo "error: unknown tool '$1'" >&2
        echo >&2
        usage >&2
        exit 1
        ;;
esac
