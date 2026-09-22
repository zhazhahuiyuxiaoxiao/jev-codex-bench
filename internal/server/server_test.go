package server

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"jev-codex-bench/internal/selector"
)

func TestMCPToolOverInMemoryTransport(t *testing.T) {
	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := New(func(_ context.Context) (selector.Result, error) {
		return selector.Result{SelectedPaths: []string{"target.go"}}, nil
	}).Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()
	list, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Tools) != 1 || list.Tools[0].Name != "rank_files" {
		t.Fatalf("unexpected tools: %+v", list.Tools)
	}
	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{Name: "rank_files", Arguments: map[string]any{}})
	if err != nil || result.IsError {
		t.Fatalf("call failed: %v, %+v", err, result)
	}
	if result.StructuredContent == nil {
		t.Fatal("missing structured output")
	}
}
