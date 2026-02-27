#!/usr/bin/env bash
#
# Diagnose common cargo build failures and suggest fixes.
# Called automatically by the Makefile when cargo build fails.
#
set -euo pipefail

RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
BOLD='\033[1m'
RESET='\033[0m'

echo ""
echo -e "${RED}${BOLD}cargo build failed.${RESET} Running diagnostics..."
echo ""

found=0

# ── Empty env vars that break cargo/rustc/cc ──────────────────────────
# These are known to cause cryptic errors when set to "" instead of unset.
MUST_NOT_BE_EMPTY=(
    CARGO_BUILD_TARGET
    MACOSX_DEPLOYMENT_TARGET
    SOURCE_DATE_EPOCH
    SDKROOT
    IPHONEOS_DEPLOYMENT_TARGET
)

unset_cmds=""
for var in "${MUST_NOT_BE_EMPTY[@]}"; do
    # ${!var+x} is set if $var is defined (even if empty)
    if [ -n "${!var+x}" ] && [ -z "${!var}" ]; then
        found=1
        echo -e "${YELLOW}▸ $var is set to an empty string.${RESET}"
        unset_cmds="${unset_cmds} ${var}"
    fi
done
if [ -n "$unset_cmds" ]; then
    echo ""
    echo -e "  ${GREEN}Fix:${RESET}  unset${unset_cmds}"
    echo ""
fi

# ── Cargo too old (edition2024) ───────────────────────────────────────
cargo_ver="$(cargo --version 2>/dev/null | awk '{print $2}' || true)"
if [ -n "$cargo_ver" ]; then
    major="$(echo "$cargo_ver" | cut -d. -f1)"
    minor="$(echo "$cargo_ver" | cut -d. -f2)"
    if [ "$major" -eq 1 ] && [ "$minor" -lt 85 ] 2>/dev/null; then
        found=1
        echo -e "${YELLOW}▸ Cargo $cargo_ver is too old (need ≥ 1.85 for edition2024).${RESET}"
        echo "  Your PATH may be picking up a Nix or system cargo instead of rustup."
        echo ""
        echo -e "  ${GREEN}Fix:${RESET}  rustup update stable && rustup default stable"
        echo "        Then ensure ~/.cargo/bin is first in your PATH:"
        echo "        export PATH=\"\$HOME/.cargo/bin:\$PATH\""
        echo ""
        echo "  Current cargo: $(which cargo 2>/dev/null || echo 'not found')"
        rustup_cargo="$(ls "$HOME"/.rustup/toolchains/stable-*/bin/cargo 2>/dev/null | head -1 || true)"
        if [ -n "$rustup_cargo" ]; then
            echo "  Rustup stable: $("$rustup_cargo" --version 2>/dev/null || echo 'unknown')"
        fi
        echo ""
    fi
fi

# ── SDKROOT points to missing directory on macOS ─────────────────────
if [ "$(uname -s)" = "Darwin" ] && [ -n "${SDKROOT+x}" ] && [ -n "$SDKROOT" ] && [ ! -d "$SDKROOT" ]; then
    found=1
    echo -e "${YELLOW}▸ SDKROOT points to a missing directory: $SDKROOT${RESET}"
    echo ""
    echo -e "  ${GREEN}Fix:${RESET}  unset SDKROOT"
    echo "        or set it to a valid SDK:"
    echo "        export SDKROOT=/Library/Developer/CommandLineTools/SDKs/MacOSX.sdk"
    echo ""
fi

# ── Summary ───────────────────────────────────────────────────────────
if [ "$found" -eq 0 ]; then
    echo "  No common issues detected. Check the cargo output above for details."
else
    echo -e "${BOLD}After applying the fix(es), re-run:${RESET}  make test"
fi
echo ""
