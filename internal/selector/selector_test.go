package selector

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"jev-codex-bench/internal/jev"
)

type fakeEvaluator struct{ request jev.Request }

func (f *fakeEvaluator) Evaluate(_ context.Context, request jev.Request) (jev.Response, error) {
	f.request = request
	var resp jev.Response
	resp.Model = jev.Model
	resp.Answers = map[string]jev.Answer{
		"file_00": {Type: "noul", Noul: 0.2},
		"file_01": {Type: "noul", Noul: 0.9},
		"file_02": {Type: "noul", Noul: 0.6},
	}
	resp.Usage.InputTokens = 42
	return resp, nil
}

func TestRankPicksTopTwoAndSkipsSymlink(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a.go", "b.go", "c.go"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("package fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(root, "a.go"), filepath.Join(root, "link.go")); err != nil {
		t.Fatal(err)
	}
	fake := &fakeEvaluator{}
	result, err := Rank(context.Background(), fake, root, "fix bug")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.SelectedPaths) != 2 || result.SelectedPaths[0] != "b.go" || result.SelectedPaths[1] != "c.go" {
		t.Fatalf("wrong ranking: %+v", result.SelectedPaths)
	}
	if len(fake.request.Questions) != 3 || result.InputTokens != 42 {
		t.Fatal("wrong request or usage")
	}
}
