package treesitter

import (
	"context"
	"path/filepath"
	"testing"

	graph "github.com/PizenLabs/lea/internal/graph/contracts"
)

func TestParseFile_Python(t *testing.T) {
	p := NewParser()
	ctx := context.Background()
	path := "../../../testdata/esc/python/simple.py"
	absPath, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}

	nodes, edges, err := p.ParseFile(ctx, absPath)
	if err != nil {
		t.Fatalf("ParseFile failed: %v (type: %T)", err, err)
	}

	relPath, _ := filepath.Rel(".", absPath)

	expectedNodes := map[string]graph.NodeType{
		"file:" + relPath:              graph.NodeModule,
		"type:" + relPath + ":Greeter": graph.NodeStruct,
		"func:" + relPath + ":main":    graph.NodeFunction,
	}

	if len(nodes) < 3 {
		t.Errorf("Expected at least 3 nodes, got %d", len(nodes))
	}

	for id, nodeType := range expectedNodes {
		found := false
		for _, n := range nodes {
			if n.ID == id {
				found = true
				if n.Type != nodeType {
					t.Errorf("Node %s has wrong type: expected %s, got %s", id, nodeType, n.Type)
				}
				break
			}
		}
		if !found {
			t.Errorf("Expected node %s not found", id)
		}
	}

	// Verify BELONGS_TO edges
	for _, id := range []string{"type:" + relPath + ":Greeter", "func:" + relPath + ":main"} {
		found := false
		for _, e := range edges {
			if e.FromID == id && e.ToID == "file:"+relPath && e.Type == graph.EdgeBelongsTo {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected BELONGS_TO edge for %s not found", id)
		}
	}
}
