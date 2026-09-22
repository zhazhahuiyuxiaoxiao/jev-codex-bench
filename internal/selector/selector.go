package selector

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"jev-codex-bench/internal/jev"
)

type RankedFile struct {
	Path  string  `json:"path"`
	Score float64 `json:"score"`
}

type Result struct {
	Model         string       `json:"model"`
	Files         []RankedFile `json:"files"`
	InputTokens   int          `json:"input_tokens"`
	SelectedPaths []string     `json:"selected_paths"`
}

type Evaluator interface {
	Evaluate(context.Context, jev.Request) (jev.Response, error)
}

func Rank(ctx context.Context, evaluator Evaluator, root, task string) (Result, error) {
	var result Result
	if strings.TrimSpace(task) == "" || len(task) > 4000 {
		return result, fmt.Errorf("task must have 1-4000 bytes")
	}
	files, err := sourceFiles(root)
	if err != nil {
		return result, err
	}
	if len(files) == 0 || len(files) > 12 {
		return result, fmt.Errorf("expected 1-12 Go files, found %d", len(files))
	}
	stateFiles := make(map[string]string, len(files))
	questions := make(map[string]jev.Question, len(files))
	for i, name := range files {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			return result, err
		}
		if len(data) > 1800 {
			data = data[:1800]
		}
		stateFiles[name] = string(data)
		questions[fmt.Sprintf("file_%02d", i)] = jev.Question{
			Type:         "noul",
			Instructions: fmt.Sprintf("Is file %q directly relevant to implementing the requested coding task?", name),
		}
	}
	resp, err := evaluator.Evaluate(ctx, jev.Request{
		Model: jev.Model,
		State: map[string]any{
			"task":  task,
			"files": stateFiles,
		},
		Questions: questions,
	})
	if err != nil {
		return result, err
	}
	result.Model = resp.Model
	result.InputTokens = resp.Usage.InputTokens
	for i, name := range files {
		result.Files = append(result.Files, RankedFile{Path: name, Score: resp.Answers[fmt.Sprintf("file_%02d", i)].Noul})
	}
	sort.Slice(result.Files, func(i, j int) bool {
		if result.Files[i].Score == result.Files[j].Score {
			return result.Files[i].Path < result.Files[j].Path
		}
		return result.Files[i].Score > result.Files[j].Score
	})
	for i := 0; i < len(result.Files) && i < 2; i++ {
		result.SelectedPaths = append(result.SelectedPaths, result.Files[i].Path)
	}
	return result, nil
}

func sourceFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if path != root && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(files)
	return files, err
}
