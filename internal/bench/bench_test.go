package bench

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"jev-codex-bench/internal/jev"
)

func TestParseCodexEvents(t *testing.T) {
	data := []byte("{\"type\":\"item.completed\",\"item\":{\"type\":\"mcp_tool_call\"}}\n" +
		"{\"type\":\"turn.completed\",\"usage\":{\"input_tokens\":120,\"cached_input_tokens\":40,\"output_tokens\":10}}\n")
	usage, calls, completed := parseCodexEvents(data)
	if !completed || calls != 1 || usage.InputTokens != 120 || usage.CachedInputTokens != 40 || usage.OutputTokens != 10 {
		t.Fatalf("wrong event parse: %+v, %d, %t", usage, calls, completed)
	}
}

func TestReportDoesNotClaimGeneralImprovement(t *testing.T) {
	report := RenderReport(Suite{})
	if !strings.Contains(report, "do not generalize") || !strings.Contains(report, "not statistically significant") {
		t.Fatal("report omitted study limitations")
	}
}

func TestCodexEnvironmentExcludesTypeSafeKey(t *testing.T) {
	got := withoutTypeSafeKey([]string{"PATH=/usr/bin", "TYPESAFE_API_KEY=secret", "GOWORK=off"})
	if len(got) != 2 || got[0] != "PATH=/usr/bin" || got[1] != "GOWORK=off" {
		t.Fatalf("unsafe environment: %#v", got)
	}
}

func TestTasksForRun(t *testing.T) {
	all, err := tasksForRun("")
	if err != nil || !slices.Equal(all, taskIDs) {
		t.Fatalf("default tasks: %v, %v", all, err)
	}
	one, err := tasksForRun("cache-ttl")
	if err != nil || !slices.Equal(one, []string{"cache-ttl"}) {
		t.Fatalf("selected task: %v, %v", one, err)
	}
	if _, err := tasksForRun("other"); err == nil {
		t.Fatal("unknown task was accepted")
	}
}

func TestUnknownTaskRejectedBeforeCredentialCheck(t *testing.T) {
	t.Setenv("TYPESAFE_API_KEY", "")
	_, err := RunAll(context.Background(), Options{
		RepoRoot: t.TempDir(), OutputDir: t.TempDir(), BudgetFile: filepath.Join(t.TempDir(), "budget.json"),
		TaskID: "other", CodexModel: "test-model",
	})
	if err == nil || !strings.Contains(err.Error(), "unknown task") {
		t.Fatalf("want task validation error, got %v", err)
	}
}

func TestSaveCodeDiffBeforeValidators(t *testing.T) {
	base := t.TempDir()
	original := filepath.Join(base, "original")
	workspace := filepath.Join(base, "workspace")
	if err := os.Mkdir(original, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(workspace, 0700); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		filepath.Join(original, "cache.go"):  "package cache\nconst ttl = 1\n",
		filepath.Join(workspace, "cache.go"): "package cache\nconst ttl = 2\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	patch, err := fixtureDiff(context.Background(), original, workspace)
	if err != nil || !strings.Contains(string(patch), "+const ttl = 2") {
		t.Fatalf("diff missing code edit: %v, %s", err, patch)
	}
	if err := os.WriteFile(filepath.Join(workspace, "hidden_test.go"), []byte("package cache"), 0600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(base, "results")
	rel, err := saveDiagnostic(out, "cache-ttl", "prefilter", "changes.patch", patch)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(out, rel))
	if err != nil || strings.Contains(string(data), "hidden_test.go") {
		t.Fatalf("saved diff includes validator or cannot be read: %v, %s", err, data)
	}
	if info, err := os.Stat(filepath.Join(out, rel)); err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("diagnostic is not private: %v, %v", info, err)
	}
}

func TestReportLinksFailedTestOutput(t *testing.T) {
	report := RenderReport(Suite{Records: []Record{{TaskID: "cache-ttl", Mode: "prefilter", Status: "fail", PatchPath: "diagnostics/cache-ttl/prefilter/changes.patch", TestOutputPath: "diagnostics/cache-ttl/prefilter/hidden-test.txt"}}})
	if !strings.Contains(report, "diagnostics/cache-ttl/prefilter/changes.patch") || !strings.Contains(report, "diagnostics/cache-ttl/prefilter/hidden-test.txt") {
		t.Fatal("report omitted diagnostic paths")
	}
}

func TestRunOnePreservesFailedHiddenTestDiagnostics(t *testing.T) {
	root := t.TempDir()
	fixture := filepath.Join(root, "fixtures", "cache-ttl")
	validator := filepath.Join(root, "_validators", "cache-ttl")
	for _, dir := range []string{fixture, validator} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for path, content := range map[string]string{
		filepath.Join(fixture, "cache.go"):         "package cache\nconst ttl = 1\n",
		filepath.Join(validator, "hidden_test.go"): "package cache\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	codexBin := filepath.Join(root, "fake-codex")
	goBin := filepath.Join(root, "fake-go")
	for path, script := range map[string]string{
		codexBin: "#!/bin/sh\nprintf 'package cache\\nconst ttl = 2\\n' > cache.go\nprintf '{\"type\":\"turn.completed\"}\\n'\n",
		goBin:    "#!/bin/sh\necho 'hidden failure marker'\nexit 1\n",
	} {
		if err := os.WriteFile(path, []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
	}
	out := filepath.Join(root, "results")
	ledger := &jev.Ledger{Path: filepath.Join(root, "budget.json")}
	record := runOne(context.Background(), Options{RepoRoot: root, OutputDir: out, CodexModel: "test-model", CodexBin: codexBin, GoBin: goBin}.defaults(), Task{ID: "cache-ttl", Prompt: "fix TTL"}, "baseline", ledger, nil)
	if record.Status != "fail" || record.DiagnosticsError != "" || record.PatchPath == "" || record.TestOutputPath == "" {
		t.Fatalf("unexpected run record: %+v", record)
	}
	patch, err := os.ReadFile(filepath.Join(out, record.PatchPath))
	if err != nil || !strings.Contains(string(patch), "+const ttl = 2") || strings.Contains(string(patch), "hidden_test.go") {
		t.Fatalf("wrong saved patch: %v, %s", err, patch)
	}
	testOutput, err := os.ReadFile(filepath.Join(out, record.TestOutputPath))
	if err != nil || !strings.Contains(string(testOutput), "hidden failure marker") {
		t.Fatalf("missing failed test output: %v, %s", err, testOutput)
	}
}

func TestHiddenValidatorDoesNotCollideWithVisibleTestName(t *testing.T) {
	root := filepath.Join("..", "..")
	for _, tc := range []struct{ task, pkg, testName string }{
		{"cache-ttl", "cachettl", "TestNonPositiveTTL"},
		{"csv-export", "csvexport", "TestFormulaCells"},
		{"claim-queue", "claimqueue", "TestConcurrentClaimUnique"},
	} {
		t.Run(tc.task, func(t *testing.T) {
			workspace := t.TempDir()
			if err := copyTree(filepath.Join(root, "fixtures", tc.task), workspace); err != nil {
				t.Fatal(err)
			}
			visibleTest := "package " + tc.pkg + "\nimport \"testing\"\nfunc " + tc.testName + "(t *testing.T) {}\n"
			if err := os.WriteFile(filepath.Join(workspace, "agent_test.go"), []byte(visibleTest), 0600); err != nil {
				t.Fatal(err)
			}
			if err := copyTree(filepath.Join(root, "_validators", tc.task), workspace); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("go", "test", "-run", "^$", "./...")
			cmd.Dir = workspace
			cmd.Env = append(withoutTypeSafeKey(os.Environ()), "GOWORK=off")
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("same-named visible and hidden tests did not compile: %v\n%s", err, output)
			}
		})
	}
}
