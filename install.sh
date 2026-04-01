#!/bin/bash
set -euo pipefail

REPO="vladzaharia/dotfiles-helpers"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
[[ "$ARCH" == "x86_64" ]] && ARCH="amd64"
[[ "$ARCH" == "aarch64" ]] && ARCH="arm64"

VERSION=$(curl -sSf "https://api.github.com/repos/$REPO/releases/latest" | grep tag_name | cut -d'"' -f4)

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
        "$0" agent-helper
        "$0" vault-helper
        "$0" sops-helper
        ;;
    *)
        echo "Usage: $0 [agent-helper|vault-helper|sops-helper|all]"
        exit 1
        ;;
esac
