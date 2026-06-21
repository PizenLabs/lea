// Package treesitter parses non-Go sources using tree-sitter.
package treesitter

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	graph "github.com/PizenLabs/lea/internal/graph/contracts"
	"github.com/PizenLabs/lea/internal/parser/treesitter/python"
	sitter "github.com/tree-sitter/go-tree-sitter"
	"github.com/tree-sitter/tree-sitter-python/bindings/go"
	"github.com/tree-sitter/tree-sitter-rust/bindings/go"
	"github.com/tree-sitter/tree-sitter-typescript/bindings/go"
)

// Parser parses non-Go sources using tree-sitter queries.
type Parser struct {
	languages map[string]*sitter.Language
	queries   map[string]string
}

// NewParser constructs a tree-sitter parser.
func NewParser() *Parser {
	return &Parser{
		languages: map[string]*sitter.Language{
			".py": sitter.NewLanguage(tree_sitter_python.Language()),
			".rs": sitter.NewLanguage(tree_sitter_rust.Language()),
			".ts": sitter.NewLanguage(tree_sitter_typescript.LanguageTypescript()),
		},
		queries: map[string]string{
			".py": python.SymbolsQuery,
		},
	}
}

// ParseFile extracts symbol nodes and edges from a source file.
func (p *Parser) ParseFile(_ context.Context, path string) ([]*graph.Node, []*graph.Edge, error) {
	ext := filepath.Ext(path)
	lang, ok := p.languages[ext]
	if !ok {
		return nil, nil, fmt.Errorf("unsupported language: %s", ext)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}

	parser := sitter.NewParser()
	defer parser.Close()
	if err := parser.SetLanguage(lang); err != nil {
		return nil, nil, fmt.Errorf("failed to set language: %w", err)
	}

	tree := parser.Parse(content, nil)
	if tree == nil {
		return nil, nil, fmt.Errorf("failed to parse %s", path)
	}
	defer tree.Close()

	var nodes []*graph.Node
	var edges []*graph.Edge

	relPath, _ := filepath.Rel(".", path)
	moduleID := fmt.Sprintf("file:%s", relPath)
	nodes = append(nodes, &graph.Node{
		ID:   moduleID,
		Type: graph.NodeModule,
		Name: filepath.Base(path),
		File: relPath,
	})

	queryStr, ok := p.queries[ext]
	if !ok {
		return nodes, edges, nil
	}

	query, qErr := sitter.NewQuery(lang, queryStr)
	if qErr != nil {
		return nil, nil, fmt.Errorf("failed to create query: %w", qErr)
	}
	defer query.Close()

	cursor := sitter.NewQueryCursor()
	captures := cursor.Captures(query, tree.RootNode(), content)
	captureNames := query.CaptureNames()

	for {
		match, _ := captures.Next()
		if match == nil {
			break
		}

		for _, capture := range match.Captures {
			captureName := captureNames[capture.Index]
			if strings.HasSuffix(captureName, ".name") {
				continue
			}

			nodeName := ""
			// Find the .name capture in the same match
			for _, c := range match.Captures {
				cn := captureNames[c.Index]
				if cn == strings.Split(captureName, ".")[0]+".name" {
					nodeName = string(content[c.Node.StartByte():c.Node.EndByte()])
					break
				}
			}

			if nodeName == "" {
				continue
			}

			nodeType := graph.NodeFunction
			prefix := "func"
			if strings.HasPrefix(captureName, "class") {
				nodeType = graph.NodeStruct
				prefix = "type"
			}

			id := fmt.Sprintf("%s:%s:%s", prefix, relPath, nodeName)
			nodes = append(nodes, &graph.Node{
				ID:   id,
				Type: nodeType,
				Name: nodeName,
				File: relPath,
				Line: int(capture.Node.StartPosition().Row) + 1,
			})

			edges = append(edges, &graph.Edge{
				FromID: id,
				ToID:   moduleID,
				Type:   graph.EdgeBelongsTo,
			})
		}
	}

	return nodes, edges, nil
}

// ExtractCalls returns call edges for the given file when supported.
func (p *Parser) ExtractCalls(_ context.Context, _ string) ([]*graph.Edge, error) {
	return nil, nil
}

// ExtractControlFlow returns control-flow edges for the given file when supported.
func (p *Parser) ExtractControlFlow(_ context.Context, _ string) ([]*graph.Edge, error) {
	return nil, nil
}
