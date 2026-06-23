#!/usr/bin/env python3
"""
mcp-bridge.py — MCP Transport Bridge (Option A)

Intercepts JSON-RPC 2.0 messages between OpenCode and MCP servers (lea/lx),
remapping double-prefixed method names before forwarding to the native binary.

Prefix mapping:
  lea_lea_view_symbol_ast   -> lea_view_symbol_ast
  lea_lea_find_neighbors    -> lea_find_neighbors
  lynx_lynx_search_graph    -> lynx_search_graph
  lynx_lynx_resolve_symbol  -> lynx_resolve_symbol

Usage:  ./mcp-bridge.py <binary-path> [args...]

OpenCode config entry (~/.opencode/settings.json):
  "mcp": {
    "pizen-lea-bridged": {
      "command": ["/path/to/mcp-bridge.py", "/path/to/lea", "mcp"],
      "enabled": true
    }
  }
"""
import json
import os
import subprocess
import sys
import threading


def remap_method(method: str) -> str:
    if method.startswith("lea_lea_"):
        return method[4:]  # lea_lea_X -> lea_X
    if method.startswith("lynx_lynx_"):
        return method[5:]  # lynx_lynx_X -> lynx_X
    if method.startswith("pizen_lea_lea_"):
        return "lea_" + method[len("pizen_lea_lea_"):]
    if method.startswith("pizen_lynx_lynx_"):
        return "lynx_" + method[len("pizen_lynx_lynx_"):]
    return method


def main():
    if len(sys.argv) < 2:
        print("Usage: mcp-bridge.py <binary-path> [args...]", file=sys.stderr)
        sys.exit(1)

    binary = sys.argv[1]
    args = sys.argv[2:]

    if not os.access(binary, os.X_OK):
        print(f"mcp-bridge: ERROR: binary not executable: {binary}", file=sys.stderr)
        sys.exit(1)

    proc = subprocess.Popen(
        [binary] + args,
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        bufsize=0,
    )

    def forward_stderr():
        for line in iter(proc.stderr.readline, b""):
            sys.stderr.buffer.write(line)
            sys.stderr.buffer.flush()

    stderr_thread = threading.Thread(target=forward_stderr, daemon=True)
    stderr_thread.start()

    def pipe_stdin_to_native():
        try:
            for raw_line in sys.stdin.buffer:
                line = raw_line.decode("utf-8", errors="replace").strip()
                if not line:
                    continue

                try:
                    msg = json.loads(line)
                except json.JSONDecodeError:
                    proc.stdin.write(raw_line)
                    proc.stdin.flush()
                    continue

                if "method" in msg:
                    original = msg["method"]
                    remapped = remap_method(original)
                    if remapped != original:
                        msg["method"] = remapped

                proc.stdin.write((json.dumps(msg) + "\n").encode())
                proc.stdin.flush()
        except BrokenPipeError:
            pass
        finally:
            proc.stdin.close()

    stdin_thread = threading.Thread(target=pipe_stdin_to_native, daemon=True)
    stdin_thread.start()

    def pipe_native_to_stdout():
        try:
            for line in iter(proc.stdout.readline, b""):
                sys.stdout.buffer.write(line)
                sys.stdout.buffer.flush()
        except BrokenPipeError:
            pass

    pipe_native_to_stdout()

    stdin_thread.join(timeout=2)
    proc.wait()


if __name__ == "__main__":
    main()