#!/usr/bin/env bash
# Dockship Linux/macOS Installer
# Usage: curl -fsSL https://raw.githubusercontent.com/dockship/dockship/main/install.sh | bash

set -e

echo "🚀 Building and installing Dockship..."

INSTALL_DIR="${HOME}/.dockship/bin"
mkdir -p "${INSTALL_DIR}"

CGO_ENABLED=0 go build -ldflags="-s -w" -o "${INSTALL_DIR}/dockship" ./cmd/dockship
chmod +x "${INSTALL_DIR}/dockship"

echo "✔ Dockship binary installed at: ${INSTALL_DIR}/dockship"

# Add to PATH in shell profile if needed
SHELL_PROFILE="${HOME}/.bashrc"
if [ -n "$ZSH_VERSION" ] || [ -f "${HOME}/.zshrc" ]; then
    SHELL_PROFILE="${HOME}/.zshrc"
fi

if ! echo "$PATH" | grep -q "${INSTALL_DIR}"; then
    echo "export PATH=\"${INSTALL_DIR}:\$PATH\"" >> "${SHELL_PROFILE}"
    echo "✔ Added ${INSTALL_DIR} to ${SHELL_PROFILE}"
    echo "ℹ Run 'source ${SHELL_PROFILE}' or restart your terminal."
fi

echo "🎉 Dockship is ready! Test it by running: dockship --help"
