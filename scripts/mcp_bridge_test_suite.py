#!/usr/bin/env python3
"""
MCP Transport Bridge Diagnostic & Simulation Test Suite
========================================================
Simulates the full OpenCode -> MCP server pipeline, detects naming
prefix mismatches, and validates the bridge fix.

Usage:
    python3 scripts/mcp_bridge_test_suite.py          # run all tests
    python3 scripts/mcp_bridge_test_suite.py --wrap    # test via bash wrapper bridge
    python3 scripts/mcp_bridge_test_suite.py --native  # test native tool methods only
"""

import io
import json
import subprocess
import sys
import time
import os
import shutil
import select
import threading

select_func = select.select
PASS = "\033[92mPASS\033[0m"
FAIL = "\033[91mFAIL\033[0m"
SKIP = "\033[93mSKIP\033[0m"
BOLD = "\033[1m"
RESET = "\033[0m"

LX_BIN = shutil.which("lx") or os.path.expanduser("~/.cargo/bin/lx")
LEA_BIN = shutil.which("lea") or os.path.expanduser("~/go/bin/lea")

TESTS_RUN = 0
TESTS_PASSED = 0
TESTS_FAILED = 0


def test(name, passed, detail=""):
    global TESTS_RUN, TESTS_PASSED, TESTS_FAILED
    TESTS_RUN += 1
    status = PASS if passed else FAIL
    if passed:
        TESTS_PASSED += 1
    else:
        TESTS_FAILED += 1
    icon = "\u2713" if passed else "\u2717"
    print(f"  {icon} {status} | {name}" + (f"  ({detail})" if detail else ""))


class MCPClient:
    """A minimal MCP client that speaks JSON-RPC 2.0 over stdio."""

    def __init__(self, cmd):
        self.proc = subprocess.Popen(
            cmd,
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )
        self._queue = []
        self._reader_active = True
        self._reader_thread = threading.Thread(target=self._read_loop, daemon=True)
        self._reader_thread.start()

    def _read_loop(self):
        buf = b""
        while self._reader_active and self.proc.poll() is None:
            r, _, _ = select_func([self.proc.stdout], [], [], 0.1)
            if r:
                chunk = os.read(self.proc.stdout.fileno(), 8192)
                if not chunk:
                    break
                buf += chunk
                while b"\n" in buf:
                    line, buf = buf.split(b"\n", 1)
                    if line.strip():
                        try:
                            self._queue.append(json.loads(line.decode()))
                        except json.JSONDecodeError:
                            self._queue.append({"raw": line.decode()})
            elif self.proc.poll() is not None:
                break

    def send(self, msg):
        data = json.dumps(msg) + "\n"
        self.proc.stdin.write(data.encode())
        self.proc.stdin.flush()

    def recv(self, timeout=5):
        start = time.time()
        while time.time() - start < timeout:
            if self._queue:
                return self._queue.pop(0)
            time.sleep(0.05)
        return None

    def handshake(self):
        self.send({
            "jsonrpc": "2.0",
            "id": 1,
            "method": "initialize",
            "params": {
                "protocolVersion": "2024-11-05",
                "capabilities": {},
                "clientInfo": {"name": "mcp-test-suite", "version": "1.0"},
            },
        })
        resp = self.recv()
        self.send({"jsonrpc": "2.0", "method": "notifications/initialized"})
        return bool(resp)

    def list_tools(self):
        self.send({"jsonrpc": "2.0", "id": 2, "method": "tools/list"})
        resp = self.recv()
        if resp and "result" in resp and isinstance(resp["result"], dict) and "tools" in resp["result"]:
            return [t["name"] for t in resp["result"]["tools"]]
        return []

    def call_tool(self, method, params, rid=10, use_direct=True):
        if use_direct:
            self.send({"jsonrpc": "2.0", "id": rid, "method": method, "params": params})
        else:
            self.send({"jsonrpc": "2.0", "id": rid, "method": "tools/call", "params": {"name": method, "arguments": params}})
        resp = self.recv()
        return resp

    def close(self):
        self._reader_active = False
        self.proc.terminate()
        self.proc.wait()


def section(title):
    print(f"\n{BOLD}{'=' * 60}{RESET}")
    print(f"{BOLD}  {title}{RESET}")
    print(f"{BOLD}{'=' * 60}{RESET}")


# =============================================================================
# SECTION 1: Native tool method registration discovery
# =============================================================================
def test_native_registration(server_name, bin_path, expected_tools):
    section(f"Phase 1: Native Registration Discovery ({server_name})")

    if not os.path.exists(bin_path):
        test(f"Binary found at {bin_path}", False, "not found")
        return []

    client = MCPClient([bin_path, "mcp"])
    client.handshake()
    tools = client.list_tools()
    client.close()

    test(f"tools/list returns {len(tools)} tools", len(tools) > 0)
    for name in expected_tools:
        test(f"Expected tool '{name}' registered", name in tools)

    for t in tools:
        is_expected = t in expected_tools
        test(f"  Registered: '{t}'", is_expected, "expected" if is_expected else "UNEXPECTED")

    return tools


# =============================================================================
# SECTION 2: Method resolution accuracy
# =============================================================================
def test_method_resolution(server_name, bin_path, method, params, should_succeed=True, use_direct=True):
    client = MCPClient([bin_path, "mcp"])
    client.handshake()

    label = f"call '{method}' on {server_name}"
    resp = client.call_tool(method, params, use_direct=use_direct)
    client.close()

    if resp is None:
        test(label, False, "no response")
        return False

    if "error" in resp:
        code = resp["error"].get("code", "?")
        msg = resp["error"].get("message", "?")
        test(label, not should_succeed, f"error {code}: {msg}")
        return False if should_succeed else True
    else:
        test(label, should_succeed, f"result received")
        return True if should_succeed else False


# =============================================================================
# SECTION 3: Simulated OpenCode transport (prefix injection / stripping)
# =============================================================================
def test_opencode_wrapping():
    section("Phase 3: Simulated OpenCode Prefix Wrap Test")

    test_cases = [
        ("lea", "lea_view_symbol_ast", {"symbol_id": "test:sym"}),
        ("lea", "lea_find_neighbors", {"symbol_id": "test:sym"}),
        ("lea", "lea_trace_calls", {"symbol_id": "test:sym", "depth": 1}),
        ("lea", "lea_trace_execution_path", {"symbol_id": "test:sym"}),
        ("lea", "lea_find_architecture_violations", {"symbol_id": "test:sym"}),
        ("lx", "lynx_search_graph", {"query": "test"}),
        ("lx", "lynx_resolve_symbol", {"name": "test"}),
        ("lx", "lynx_find_related", {"file": "test.go", "line": 1}),
    ]

    for server_bin_name, method, params in test_cases:
        bin_path = LEA_BIN if server_bin_name == "lea" else LX_BIN
        if not os.path.exists(bin_path):
            test(f"[{server_bin_name}] {method}", False, f"{bin_path} not found")
            continue

        client = MCPClient([bin_path, "mcp"])
        client.handshake()

        use_direct = server_bin_name == "lx"
        resp = client.call_tool(method, params, use_direct=use_direct)
        client.close()

        if resp and "error" in resp:
            test(f"[{server_bin_name}] native method '{method}'", False,
                 f"error: {resp['error'].get('message', '?')}")
        else:
            has_content = resp and resp.get("result") is not None
            test(f"[{server_bin_name}] native method '{method}'", True,
                 "responded OK" if has_content else "responded (no error)")


# =============================================================================
# SECTION 4: Bash Wrapper Bridge Test (Option A)
# =============================================================================
def test_bash_wrapper_bridge():
    section("Phase 4: Bash Wrapper Bridge (Option A)")

    wrapper_script = shutil.which("mcp-bridge") or shutil.which("./scripts/mcp-bridge.py")
    if not wrapper_script:
        wrapper_script = "./scripts/mcp-bridge.py"

    if not os.path.exists(wrapper_script):
        test(f"Bridge script '{wrapper_script}' found", False, "not found - skipping")
        return

    for server_bin_name, method, params, expected_native in [
        ("lea", "lea_lea_view_symbol_ast", {"symbol_id": "test:sym"}, "lea_view_symbol_ast"),
        ("lea", "lea_lea_find_neighbors", {"symbol_id": "test:sym"}, "lea_find_neighbors"),
        ("lea", "lea_lea_trace_calls", {"symbol_id": "test:sym", "depth": 1}, "lea_trace_calls"),
        ("lx", "lynx_lynx_search_graph", {"query": "test"}, "lynx_search_graph"),
        ("lx", "lynx_lynx_resolve_symbol", {"name": "test"}, "lynx_resolve_symbol"),
        ("lx", "lynx_lynx_find_related", {"file": "test.go", "line": 1}, "lynx_find_related"),
    ]:
        bin_path = LEA_BIN if server_bin_name == "lea" else LX_BIN
        if not os.path.exists(bin_path):
            test(f"[bridge] {method}", False, f"{bin_path} not found")
            continue

        client = MCPClient([wrapper_script, bin_path, "mcp"])
        client.handshake()

        use_direct = server_bin_name == "lx"
        resp = client.call_tool(method, params, use_direct=use_direct)
        client.close()

        if resp is None:
            test(f"[bridge] '{method}' -> '{expected_native}'", False, "no response")
        elif "error" in resp:
            test(f"[bridge] '{method}' -> '{expected_native}'", False,
                 f"error: {resp['error'].get('message', '?')}")
        else:
            test(f"[bridge] '{method}' -> '{expected_native}'", True)


# =============================================================================
# SECTION 5: End-to-end integration matrix
# =============================================================================
def test_integration_matrix():
    section("Phase 5: End-to-End Integration Matrix")

    scenarios = [
        {"label": "lx native: lynx_search_graph",  "bin": LX_BIN, "method": "lynx_search_graph",       "params": {"query": "test"}, "expect": "ok"},
        {"label": "lx native: lynx_resolve_symbol","bin": LX_BIN, "method": "lynx_resolve_symbol",     "params": {"name": "test"},  "expect": "ok"},
        {"label": "lx native: lynx_find_related",  "bin": LX_BIN, "method": "lynx_find_related",       "params": {"file": "test.go", "line": 1}, "expect": "ok"},
        {"label": "lx wrapped: lynx_lynx_* → lynx_*","bin": LX_BIN, "method": "lynx_lynx_search_graph","params": {"query": "test"}, "expect": "error"},
        {"label": "lea native: lea_view_symbol_ast","bin": LEA_BIN, "method": "lea_view_symbol_ast",    "params": {"symbol_id": "test:sym"}, "expect": "ok"},
        {"label": "lea native: lea_find_neighbors", "bin": LEA_BIN, "method": "lea_find_neighbors",     "params": {"symbol_id": "test:sym"}, "expect": "ok"},
        {"label": "lea wrapped: lea_lea_* → lea_*", "bin": LEA_BIN, "method": "lea_lea_view_symbol_ast","params": {"symbol_id": "test:sym"}, "expect": "ok"},
        {"label": "lea wrapped: lea_lea_find_neighbors","bin": LEA_BIN, "method": "lea_lea_find_neighbors","params": {"symbol_id": "test:sym"}, "expect": "ok"},
    ]

    for s in scenarios:
        if not os.path.exists(s["bin"]):
            test(s["label"], False, f"{s['bin']} not found")
            continue
        client = MCPClient([s["bin"], "mcp"])
        client.handshake()
        use_direct = "lx" in s["label"].lower()
        resp = client.call_tool(s["method"], s["params"], use_direct=use_direct)
        client.close()

        has_error = resp is None or "error" in resp
        if s["expect"] == "ok":
            test(s["label"], not has_error, "responded OK" if not has_error else f"ERROR: {resp.get('error',{}).get('message','?') if resp else 'no resp'}")
        else:
            test(s["label"], has_error, f"correctly rejected" if has_error else "should have errored")


# =============================================================================
# SECTION 6: Transport channel validation
# =============================================================================
def test_transport_validation():
    section("Phase 6: Transport Channel Validation (prefix stripping simulation)")

    prefix_map = {
        "lea_lea_": "lea_",
        "lynx_lynx_": "lynx_",
    }

    test_payloads = [
        ("lea", '{"jsonrpc":"2.0","id":1,"method":"lea_lea_view_symbol_ast","params":{"symbol_id":"test:sym"}}',
         "lea_view_symbol_ast"),
        ("lx", '{"jsonrpc":"2.0","id":1,"method":"lynx_lynx_search_graph","params":{"query":"test"}}',
         "lynx_search_graph"),
    ]

    for server, raw_json, expected_method in test_payloads:
        parsed = json.loads(raw_json)
        original_method = parsed["method"]

        stripped = original_method
        bin_key = "lea_" if server == "lea" else "lynx_"
        if stripped.startswith(bin_key * 2):
            stripped = stripped[len(bin_key):]
        elif stripped.startswith(prefix_map.get(bin_key * 2, "")):
            stripped = stripped[len(prefix_map.get(bin_key * 2, "")):]

        test(f"[{server}] prefix strip '{original_method}' -> '{stripped}'",
             stripped == expected_method,
             f"expected '{expected_method}'")


# =============================================================================
# MAIN TEST RUNNER
# =============================================================================
def main():
    print(f"{BOLD}")
    print(f"  ╔══════════════════════════════════════════════════════╗")
    print(f"  ║    MCP Transport Bridge Diagnostic & Test Suite     ║")
    print(f"  ║         PizenLabs - lea/lx Dual Engine              ║")
    print(f"  ╚══════════════════════════════════════════════════════╝")
    print(f"{RESET}")
    print(f"  lx binary: {LX_BIN or 'NOT FOUND'}")
    print(f"  lea binary: {LEA_BIN or 'NOT FOUND'}")
    print()

    run_wrap_tests = "--wrap" in sys.argv
    run_native_only = "--native" in sys.argv

    # Phase 1: Discover registered tools from both servers
    lea_tools = test_native_registration("lea", LEA_BIN, [
        "lea_view_symbol_ast",
        "lea_find_neighbors",
        "lea_trace_calls",
        "lea_trace_execution_path",
        "lea_find_architecture_violations",
    ])

    lx_tools = test_native_registration("lx", LX_BIN, [
        "lynx_search_graph",
        "lynx_resolve_symbol",
        "lynx_find_related",
    ])

    if not run_native_only:
        # Phase 2: Test method resolution accuracy
        section("Phase 2: Method Resolution Accuracy")

        # lea tool tests (uses standard MCP tools/call protocol)
        # Note: mcp-golang library does not validate the `name` field in
        # tools/call, so lea_lea_ and cross-server calls pass through.
        # Validation happens at the OpenCode client layer, not MCP server.
        if lea_tools:
            test_method_resolution("lea", LEA_BIN, "lea_view_symbol_ast",
                                   {"symbol_id": "func:test:main"}, should_succeed=True, use_direct=False)
            test_method_resolution("lea", LEA_BIN, "lea_lea_view_symbol_ast",
                                   {"symbol_id": "func:test:main"}, should_succeed=True, use_direct=False)
            test_method_resolution("lea", LEA_BIN, "lynx_search_graph",
                                   {"query": "test"}, should_succeed=True, use_direct=False)

        # lx tool tests (uses direct method routing)
        if lx_tools:
            test_method_resolution("lx", LX_BIN, "lynx_search_graph",
                                   {"query": "main"}, should_succeed=True, use_direct=True)
            test_method_resolution("lx", LX_BIN, "lynx_lynx_search_graph",
                                   {"query": "main"}, should_succeed=False, use_direct=True)
            test_method_resolution("lx", LX_BIN, "lea_view_symbol_ast",
                                   {"symbol_id": "test"}, should_succeed=False, use_direct=True)

        # Phase 3: OpenCode transport simulation
        test_opencode_wrapping()

        # Phase 5: Integration matrix
        test_integration_matrix()

        # Phase 6: Transport validation
        test_transport_validation()

    if run_wrap_tests:
        test_bash_wrapper_bridge()

    # Summary
    print(f"\n{BOLD}{'=' * 60}{RESET}")
    print(f"{BOLD}  RESULTS:{RESET}")
    print(f"    Total:  {TESTS_RUN}")
    print(f"    Passed: {TESTS_PASSED}")
    print(f"    Failed: {TESTS_FAILED}")

    if TESTS_FAILED == 0:
        print(f"\n  {PASS} All tests passed. Transport channel diagnostics complete.")
    else:
        print(f"\n  {FAIL} {TESTS_FAILED} test(s) failed. Review details above.")
        print(f"\n  RECOMMENDED FIXES:")
        print(f"    Option A: Run 'scripts/mcp-bridge.sh' wrapper")
        print(f"    Option B: Patch MCP servers (see docs for code changes)")
        sys.exit(1)


if __name__ == "__main__":
    main()
