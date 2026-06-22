package commands

import (
    "bytes"
    "io"
    "os"
    "testing"
)

// Mockable os.Exit for testing
var osExitOriginal = os.Exit
var osExit = os.Exit

func TestHookPreToolInvalidSymbol(t *testing.T) {
    // Prepare JSON input with a non-existent symbol
    jsonInput := []byte(`{"tool_name":"pizen-lea__impact","tool_input":{"symbol":"nonexistent_symbol"}}`)
    // Replace stdin with pipe containing JSON input
    oldStdin := os.Stdin
    r, w, err := os.Pipe()
    if err != nil {
        t.Fatalf("pipe error: %v", err)
    }
    _, err = w.Write(jsonInput)
    if err != nil {
        t.Fatalf("write error: %v", err)
    }
    w.Close()
    os.Stdin = r
    // Capture exit code via mocking os.Exit
    exitCode := 0
    osExit = func(code int) {
        exitCode = code
        panic("os.Exit called")
    }
    defer func() {
        // Restore globals
        os.Stdin = oldStdin
        os.Exit = osExitOriginal
        if r := recover(); r != nil {
            // expected panic from os.Exit
        }
        if exitCode != 2 {
            t.Fatalf("expected exit code 2, got %d", exitCode)
        }
    }()
    // Run the pre-tool hook command
    hookPreToolCmd.Run(nil, []string{})
}
