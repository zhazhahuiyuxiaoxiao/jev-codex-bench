package broker

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"jev-codex-bench/internal/jev"
)

type fakeEvaluator struct{}

func (fakeEvaluator) Evaluate(_ context.Context, req jev.Request) (jev.Response, error) {
	var resp jev.Response
	resp.Model = jev.Model
	resp.Answers = map[string]jev.Answer{}
	for id := range req.Questions {
		resp.Answers[id] = jev.Answer{Type: "noul", Noul: 0.7}
	}
	resp.Usage.InputTokens = 20
	return resp, nil
}

func TestBrokerRanksOnlyItsWorkspace(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "public.go"), []byte("package fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	socket, stop, err := Start(context.Background(), root, "fix public code", fakeEvaluator{})
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	result, err := Rank(context.Background(), socket)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.SelectedPaths) != 1 || result.SelectedPaths[0] != "public.go" {
		t.Fatalf("unexpected result: %+v", result)
	}
}
