#!/data/data/com.termux/files/usr/bin/bash
# ──────────────────────────────────────────────────────────────────────────────
# opencode — Termux installer / builder
# Supports NVIDIA NIM, Groq, Anthropic, OpenAI, and other providers.
# ──────────────────────────────────────────────────────────────────────────────
set -e

RESET="\033[0m"; BOLD="\033[1m"
PURPLE="\033[38;2;153;153;204m"; GREEN="\033[38;2;123;204;68m"
RED="\033[38;2;255;112;112m"; YELLOW="\033[33m"; DIM="\033[2m"

info()  { echo -e "${PURPLE}${BOLD}→${RESET} $*"; }
ok()    { echo -e "${GREEN}${BOLD}✓${RESET} $*"; }
warn()  { echo -e "${YELLOW}${BOLD}!${RESET} $*"; }
die()   { echo -e "${RED}${BOLD}✗${RESET} $*"; exit 1; }

echo -e "\n${BOLD}${PURPLE}opencode — Termux installer${RESET}\n"

# ── 1. deps ──────────────────────────────────────────────────────────────────
info "Installing Termux packages..."
pkg install -y golang git 2>/dev/null || die "pkg install failed. Run: pkg update && pkg upgrade first."
ok "golang + git installed"

# ── 2. build ─────────────────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

info "Building opencode (CGO_ENABLED=0, GOOS=linux, GOARCH=arm64)..."
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
  -ldflags="-s -w" \
  -o opencode \
  ./main.go
ok "Build successful → ${SCRIPT_DIR}/opencode"

# ── 3. install ────────────────────────────────────────────────────────────────
INSTALL_DIR="$PREFIX/bin"
cp opencode "$INSTALL_DIR/opencode"
chmod +x "$INSTALL_DIR/opencode"
ok "Installed to $INSTALL_DIR/opencode"

# ── 4. config hint ───────────────────────────────────────────────────────────
CONFIG_DIR="$HOME/.config/opencode"
mkdir -p "$CONFIG_DIR"

if [ ! -f "$CONFIG_DIR/config.json" ]; then
  cat > "$CONFIG_DIR/config.json" <<'EOF'
{
  "providers": {
    "nvidia": {
      "apiKey": ""
    }
  }
}
EOF
  warn "Created $CONFIG_DIR/config.json — add your NVIDIA_API_KEY"
fi

echo ""
echo -e "${BOLD}Usage:${RESET}"
echo -e "  ${DIM}# Set your NVIDIA NIM API key (get one free at build.nvidia.com):${RESET}"
echo -e "  export NVIDIA_API_KEY=\"nvapi-xxxx\""
echo ""
echo -e "  ${DIM}# Or any other provider:${RESET}"
echo -e "  export ANTHROPIC_API_KEY=\"sk-ant-xxxx\""
echo -e "  export OPENAI_API_KEY=\"sk-xxxx\""
echo -e "  export GROQ_API_KEY=\"gsk_xxxx\""
echo ""
echo -e "  ${DIM}# Run:${RESET}"
echo -e "  opencode"
echo ""
echo -e "${GREEN}${BOLD}Done!${RESET} Run ${BOLD}opencode${RESET} to start.\n"
