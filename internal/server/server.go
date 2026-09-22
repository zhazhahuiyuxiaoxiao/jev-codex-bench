package server

import (
	"context"
	"fmt"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"jev-codex-bench/internal/selector"
)

type RankInput struct{}

type Ranker func(context.Context) (selector.Result, error)

func Run(ctx context.Context, ranker Ranker) error {
	server := New(ranker)
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		log.Printf("MCP server failed: %v", err)
		return err
	}
	return nil
}

func New(ranker Ranker) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "jev-codex-bench", Version: "0.1.0"}, &mcp.ServerOptions{
		Instructions: "Use rank_files only when file relevance is uncertain. It sends excerpts of this public fixture to TypeSafe Jev and consumes a capped credit budget. Its scores are hints, not proof; inspect code and run tests yourself.",
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "rank_files",
		Description: "Rank up to 12 Go files in this public fixture for a coding task using Jev. Returns top two paths and all probabilities.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ RankInput) (*mcp.CallToolResult, selector.Result, error) {
		res, err := ranker(ctx)
		if err != nil {
			return nil, selector.Result{}, fmt.Errorf("rank_files: %w", err)
		}
		return nil, res, nil
	})
	return server
}
