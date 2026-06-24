#!/usr/bin/env bash
# ============================================================================
# mcp-bridge.sh — MCP Transport Bridge (Option A)
#
# Sits between the OpenCode client and native MCP servers (lea/lx),
# intercepting JSON-RPC 2.0 messages over stdio and remapping method names
# by stripping the redundant double-prefix before forwarding to the binary.
#
# Prefix mapping:
#   lea_lea_view_symbol_ast   -> lea_view_symbol_ast
#   lea_lea_find_neighbors    -> lea_find_neighbors
#   lynx_lynx_search_graph    -> lynx_search_graph
#   lynx_lynx_resolve_symbol  -> lynx_resolve_symbol
#
# Usage:  ./mcp-bridge.sh <binary-path> [args...]
#
# Example (opencode.json entry):
#   "pizen-lea-bridged": {
#     "command": ["/path/to/mcp-bridge.sh", "/path/to/lea", "mcp"],
#     "enabled": true
#   }
# ============================================================================
set -euo pipefail

BINARY="$1"
shift

if [ ! -x "$BINARY" ]; then
    echo "mcp-bridge: ERROR: binary not found or not executable: $BINARY" >&2
    exit 1
fi

# Create temp directory for named pipes
TMPDIR=$(mktemp -d "${TMPDIR:-/tmp}/mcp-bridge.XXXXXXXX")
trap 'rm -rf "$TMPDIR"; kill "$NATIVE_PID" 2>/dev/null || true' EXIT INT TERM

NATIVE_IN="$TMPDIR/native_in"
NATIVE_OUT="$TMPDIR/native_out"

mkfifo "$NATIVE_IN" "$NATIVE_OUT"

# Launch native binary with piped stdio
"$BINARY" "$@" < "$NATIVE_IN" > "$NATIVE_OUT" &
NATIVE_PID=$!

# Open FIFOs for reading/writing in background
exec 3>"$NATIVE_IN"   # write to native stdin
exec 4<"$NATIVE_OUT"  # read from native stdout

# --- stdin relay: remap methods and forward to native ---
stdin_relay() {
    while IFS= read -r line; do
        [ -z "$line" ] && continue

        # Use python3 for JSON-aware method remapping
        if remapped=$(echo "$line" | python3 -c "
import json, sys
try:
    d = json.loads(sys.stdin.read())
    if 'method' in d:
        m = d['method']
        if m.startswith('lea_lea_'):
            d['method'] = m[4:]
        elif m.startswith('lynx_lynx_'):
            d['method'] = m[5:]
        elif m.startswith('pizen_lea_lea_'):
            d['method'] = 'lea_' + m[len('pizen_lea_lea_'):]
        elif m.startswith('pizen_lynx_lynx_'):
            d['method'] = 'lynx_' + m[len('pizen_lynx_lynx_'):]
    sys.stdout.write(json.dumps(d) + '\n')
except Exception:
    sys.stdout.write(line + '\n')
" 2>/dev/null); then
            echo "$remapped" >&3
        else
            echo "$line" >&3
        fi
    done
    exec 3>&-
}

# --- stdout relay: forward native responses to stdout ---
stdout_relay() {
    while IFS= read -r line <&4; do
        echo "$line"
    done
    exec 4<&-
}

# Run both relays in parallel
stdin_relay &
RELAY_PID=$!
stdout_relay

wait "$RELAY_PID" 2>/dev/null || true