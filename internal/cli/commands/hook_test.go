package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PizenLabs/lea/internal/storage/sqlite"
)

func TestHookPreToolInvalidSymbol(t *testing.T) {
	// Create a temp directory with a .lea directory and graph.db so the code
	// proceeds past findLeaDir / sqlite.NewStore into resolveSymbolID.
	tmpDir := t.TempDir()
	leaDir := filepath.Join(tmpDir, ".lea")
	if err := os.MkdirAll(leaDir, 0755); err != nil {
		t.Fatalf("mkdir .lea: %v", err)
	}

	// Create a valid empty graph.db so sqlite.NewStore opens successfully.
	dbPath := filepath.Join(leaDir, "graph.db")
	store, err := sqlite.NewStore(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	store.Close()

	// Prepare JSON input with a non-existent symbol
	jsonInput := []byte(`{"tool_name":"pizen-lea__impact","tool_input":{"symbol":"nonexistent_symbol"}}`)

	oldStdin := os.Stdin
	oldWd, _ := os.Getwd()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe error: %v", err)
	}
	_, err = w.Write(jsonInput)
	if err != nil {
		t.Fatalf("write error: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close error: %v", err)
	}

	os.Stdin = r
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	exitCode := 0
	osExit = func(code int) {
		exitCode = code
		panic("os.Exit called")
	}
	defer func() {
		os.Stdin = oldStdin
		os.Chdir(oldWd)
		osExit = osExitOriginal
		_ = recover()
		if exitCode != 2 {
			t.Fatalf("expected exit code 2, got %d", exitCode)
		}
	}()
	hookPreToolCmd.Run(nil, []string{})
}
