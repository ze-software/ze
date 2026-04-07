#!/usr/bin/env bash
# Setup script for ze development environment on Ubuntu
# Run as root: sudo bash setup_claude_server.sh [username]

set -euo pipefail

# --- Configuration ---
GO_VERSION="1.26.0"
NODE_MAJOR=22
ZE_REPO="https://codeberg.org/thomas-mangin/ze.git"
TARGET_USER="${1:-thomas}"
TARGET_HOME="/home/${TARGET_USER}"
SSH_KEY_DIR="${2:-}"  # local path to SSH key directory (contains id_ed25519 + id_ed25519.pub)

# --- Sanity checks ---
if [[ $EUID -ne 0 ]]; then
    echo "error: run as root (sudo bash $0 $TARGET_USER)" >&2
    exit 1
fi

if ! id "$TARGET_USER" &>/dev/null; then
    echo "error: user '$TARGET_USER' does not exist" >&2
    exit 1
fi

echo "=== Setting up ze dev environment for user: $TARGET_USER ==="

# --- 1. System packages ---
echo ""
echo "--- Installing system packages ---"
apt-get update -qq
apt-get install -y -qq build-essential git curl wget jq unzip mosh

# --- 1b. Firewall ---
echo ""
echo "--- Configuring firewall ---"
if command -v ufw &>/dev/null; then
    # Allow SSH before enabling — critical to avoid lockout
    ufw allow OpenSSH
    ufw allow 60000:61000/udp comment 'mosh'
    if ufw status | grep -q active; then
        echo "ufw already active, rules updated"
    else
        echo "y" | ufw enable
        echo "ufw enabled"
    fi
    ufw status
else
    echo "ufw not found, skipping firewall setup"
fi

# --- 2. Go ---
echo ""
echo "--- Installing Go ${GO_VERSION} ---"
if [[ -x /usr/local/go/bin/go ]] && /usr/local/go/bin/go version | grep -q "go${GO_VERSION}"; then
    echo "Go ${GO_VERSION} already installed, skipping"
else
    curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -o /tmp/go.tar.gz
    rm -rf /usr/local/go
    tar -C /usr/local -xzf /tmp/go.tar.gz
    rm /tmp/go.tar.gz
    echo "Go $(/usr/local/go/bin/go version) installed"
fi

# --- 2b. Go tools ---
echo ""
echo "--- Installing Go tools ---"
export PATH="/usr/local/go/bin:${TARGET_HOME}/go/bin:$PATH"
export GOPATH="${TARGET_HOME}/go"

sudo -u "$TARGET_USER" mkdir -p "${GOPATH}/bin"

if command -v golangci-lint &>/dev/null; then
    echo "golangci-lint already installed: $(golangci-lint --version 2>/dev/null | head -1)"
else
    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b "${GOPATH}/bin"
    chown -R "$TARGET_USER:$TARGET_USER" "${GOPATH}"
    echo "golangci-lint installed: $(golangci-lint --version 2>/dev/null | head -1)"
fi

if command -v goimports &>/dev/null; then
    echo "goimports already installed"
else
    sudo -u "$TARGET_USER" /usr/local/go/bin/go install golang.org/x/tools/cmd/goimports@latest
    echo "goimports installed"
fi

# --- 3. Node.js (for Claude CLI) ---
echo ""
echo "--- Installing Node.js ${NODE_MAJOR}.x ---"
if command -v node &>/dev/null && node --version | grep -q "^v${NODE_MAJOR}\."; then
    echo "Node.js $(node --version) already installed, skipping"
else
    curl -fsSL "https://deb.nodesource.com/setup_${NODE_MAJOR}.x" | bash -
    apt-get install -y -qq nodejs
    echo "Node.js $(node --version) installed"
fi

# --- 4. Claude CLI ---
echo ""
echo "--- Installing Claude CLI ---"
if command -v claude &>/dev/null; then
    echo "Claude CLI already installed, skipping"
else
    npm install -g @anthropic-ai/claude-code
    echo "Claude CLI installed: $(claude --version 2>/dev/null || echo 'ok')"
fi

# --- 5. Shell environment for target user ---
echo ""
echo "--- Configuring shell environment ---"
BASHRC="${TARGET_HOME}/.bashrc"

if ! grep -q '/usr/local/go/bin' "$BASHRC" 2>/dev/null; then
    cat >> "$BASHRC" << 'GOENV'

# Go environment
export PATH="/usr/local/go/bin:$HOME/go/bin:$PATH"
export GOPATH="$HOME/go"
export COLORTERM=truecolor
GOENV
    echo "Added Go paths to .bashrc"
else
    echo "Go paths already in .bashrc"
fi

# Shell aliases
if ! grep -q 'alias cc=' "$BASHRC" 2>/dev/null; then
    cat >> "$BASHRC" << 'ALIASES'

# Claude alias
alias cc='claude --dangerously-skip-permissions'
ALIASES
    echo "Added cc alias to .bashrc"
else
    echo "cc alias already in .bashrc"
fi

# SSH agent — persist across mosh sessions using fixed socket
if ! grep -q 'SSH_AUTH_SOCK.*agent.sock' "$BASHRC" 2>/dev/null; then
    cat >> "$BASHRC" << 'SSHAGENT'

# SSH agent — persist across mosh sessions using fixed socket
export SSH_AUTH_SOCK="$HOME/.ssh/agent.sock"
if ! ssh-add -l &>/dev/null; then
    rm -f "$SSH_AUTH_SOCK"
    eval "$(ssh-agent -a "$SSH_AUTH_SOCK" -s)" > /dev/null
fi
SSHAGENT
    echo "Added ssh-agent to .bashrc"
else
    echo "ssh-agent already in .bashrc"
fi

# Auto cd to ze repo on login
if ! grep -q 'cd.*ze/main' "$BASHRC" 2>/dev/null; then
    cat >> "$BASHRC" << 'AUTOCD'

# Land in ze dev repo on login
cd ~/Code/codeberg.org/thomas-mangin/ze/main 2>/dev/null
AUTOCD
    echo "Added auto-cd to .bashrc"
else
    echo "Auto-cd already in .bashrc"
fi

# --- 6. SSH key for Codeberg ---
echo ""
echo "--- SSH key setup ---"
SSH_DIR="${TARGET_HOME}/.ssh"
KEY_FILE="${SSH_DIR}/id_ed25519"

if [[ -f "$KEY_FILE" ]]; then
    echo "SSH key already exists at $KEY_FILE"
elif [[ -n "$SSH_KEY_DIR" && -f "$SSH_KEY_DIR/id_ed25519" ]]; then
    echo "Copying SSH key from $SSH_KEY_DIR..."
    cp "$SSH_KEY_DIR/id_ed25519" "$KEY_FILE"
    cp "$SSH_KEY_DIR/id_ed25519.pub" "${KEY_FILE}.pub"
    chown "$TARGET_USER:$TARGET_USER" "$KEY_FILE" "${KEY_FILE}.pub"
    chmod 600 "$KEY_FILE"
    chmod 644 "${KEY_FILE}.pub"
    echo "SSH key installed: $(cat "${KEY_FILE}.pub")"
else
    echo "error: no SSH key found and no key directory provided" >&2
    echo "usage: $0 $TARGET_USER /path/to/ssh-key-dir" >&2
    exit 1
fi

# --- 7. Clone ze repo ---
echo ""
echo "--- Repository setup ---"
ZE_DIR="${TARGET_HOME}/Code/codeberg.org/thomas-mangin/ze/main"

if [[ -d "$ZE_DIR/.git" ]]; then
    echo "ze repo already cloned at $ZE_DIR"
else
    echo "Creating directory structure..."
    sudo -u "$TARGET_USER" mkdir -p "$(dirname "$ZE_DIR")"

    echo "Cloning ze repo (HTTPS)..."
    if sudo -u "$TARGET_USER" git clone "$ZE_REPO" "$ZE_DIR"; then
        echo "Cloned successfully"
    else
        echo ""
        echo "CLONE FAILED — add SSH key to Codeberg first, then run:"
        echo "  git clone git@codeberg.org:thomas-mangin/ze.git $ZE_DIR"
    fi
fi

# --- 8. Verify ---
echo ""
echo "=== Verification ==="
echo -n "Go:       "; /usr/local/go/bin/go version 2>/dev/null || echo "MISSING"
echo -n "Node:     "; node --version 2>/dev/null || echo "MISSING"
echo -n "npm:      "; npm --version 2>/dev/null || echo "MISSING"
echo -n "Claude:   "; claude --version 2>/dev/null || echo "MISSING"
echo -n "Make:     "; make --version 2>/dev/null | head -1 || echo "MISSING"
echo -n "Lint:     "; golangci-lint --version 2>/dev/null | head -1 || echo "MISSING"
echo -n "Imports:  "; command -v goimports &>/dev/null && echo "$(goimports --help 2>&1 | head -1)" || echo "MISSING"
echo -n "Git:      "; git --version 2>/dev/null || echo "MISSING"
echo -n "Mosh:     "; mosh-server --version 2>/dev/null | head -1 || echo "MISSING"
echo -n "Firewall: "; ufw status 2>/dev/null | head -1 || echo "MISSING"
echo -n "Ze repo:  "; [[ -d "$ZE_DIR/.git" ]] && echo "$ZE_DIR" || echo "NOT CLONED"

echo ""
echo "=== Done ==="
echo ""
echo "Next steps:"
echo "  1. Log in as $TARGET_USER (or: su - $TARGET_USER)"
echo "  2. Run 'claude' to authenticate and start working"
echo "  3. cd $ZE_DIR && make ze-verify"
