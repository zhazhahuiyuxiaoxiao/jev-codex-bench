package bench

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"jev-codex-bench/internal/broker"
	"jev-codex-bench/internal/jev"
	"jev-codex-bench/internal/selector"
)

var taskIDs = []string{"csv-export", "cache-ttl", "claim-queue"}
var modes = []string{"baseline", "prefilter", "autonomous"}

type Task struct {
	ID     string `json:"id"`
	Prompt string `json:"prompt"`
}

type CodexUsage struct {
	InputTokens           int `json:"input_tokens"`
	CachedInputTokens     int `json:"cached_input_tokens"`
	OutputTokens          int `json:"output_tokens"`
	ReasoningOutputTokens int `json:"reasoning_output_tokens"`
}

type Record struct {
	TaskID       string     `json:"task_id"`
	Mode         string     `json:"mode"`
	Status       string     `json:"status"`
	Error        string     `json:"error,omitempty"`
	ElapsedMS    int64      `json:"elapsed_ms"`
	CodexUsage   CodexUsage `json:"codex_usage"`
	MCPCalls     int        `json:"mcp_calls"`
	JevCalls     int        `json:"jev_calls"`
	JevUSD       float64    `json:"jev_usd"`
	Selected     []string   `json:"selected,omitempty"`
	CodexVersion string     `json:"codex_version"`
	CodexModel   string     `json:"codex_model_requested,omitempty"`
	JevModel     string     `json:"jev_model"`
	TestsPassed  bool       `json:"tests_passed"`
	FinishedAt   time.Time  `json:"finished_at"`
}

type Suite struct {
	GeneratedAt time.Time       `json:"generated_at"`
	Records     []Record        `json:"records"`
	Budget      jev.LedgerState `json:"budget"`
}

type Options struct {
	RepoRoot   string
	OutputDir  string
	BudgetFile string
	CodexModel string
	CodexBin   string
	GoBin      string
	Timeout    time.Duration
}

func (o Options) defaults() Options {
	if o.CodexBin == "" {
		o.CodexBin = "codex"
	}
	if o.GoBin == "" {
		o.GoBin = "go"
	}
	if o.Timeout == 0 {
		o.Timeout = 8 * time.Minute
	}
	return o
}

func CheckFixtures(ctx context.Context, repoRoot, goBin string) error {
	if goBin == "" {
		goBin = "go"
	}
	for _, id := range taskIDs {
		root, err := os.MkdirTemp("", "jev-fixture-check-*")
		if err != nil {
			return err
		}
		if err := copyTree(filepath.Join(repoRoot, "fixtures", id), root); err != nil {
			os.RemoveAll(root)
			return err
		}
		visible := exec.CommandContext(ctx, goBin, "test", "./...")
		visible.Dir = root
		visible.Env = append(withoutTypeSafeKey(os.Environ()), "GOWORK=off")
		if err := visible.Run(); err != nil {
			os.RemoveAll(root)
			return fmt.Errorf("fixture %s visible tests failed before repair: %w", id, err)
		}
		if err := copyTree(filepath.Join(repoRoot, "_validators", id), root); err != nil {
			os.RemoveAll(root)
			return err
		}
		args := []string{"test", "./..."}
		if id == "claim-queue" {
			args = []string{"test", "-race", "./..."}
		}
		cmd := exec.CommandContext(ctx, goBin, args...)
		cmd.Dir = root
		cmd.Env = append(withoutTypeSafeKey(os.Environ()), "GOWORK=off")
		err = cmd.Run()
		os.RemoveAll(root)
		if err == nil {
			return fmt.Errorf("fixture %s unexpectedly passes its hidden tests before a fix", id)
		}
	}
	return nil
}

func RunAll(ctx context.Context, raw Options) (Suite, error) {
	o := raw.defaults()
	var suite Suite
	if o.RepoRoot == "" || o.OutputDir == "" || o.BudgetFile == "" {
		return suite, errors.New("repo root, output directory and budget file are required")
	}
	if o.CodexModel == "" {
		return suite, errors.New("--codex-model is required so all arms use one pinned model")
	}
	ledger := &jev.Ledger{Path: o.BudgetFile}
	client := jev.NewClient(ledger)
	if client.Key == "" {
		return suite, errors.New("TYPESAFE_API_KEY is missing; no experiments were started")
	}
	// Keep the credential only in this process's client, never in subprocess
	// environments or a reusable MCP configuration.
	if err := os.Unsetenv("TYPESAFE_API_KEY"); err != nil {
		return suite, err
	}
	if err := os.MkdirAll(o.OutputDir, 0700); err != nil {
		return suite, err
	}
	if _, err := os.Stat(filepath.Join(o.OutputDir, "results.json")); err == nil {
		return suite, errors.New("results.json already exists; use a new output directory to preserve previous results")
	} else if !errors.Is(err, os.ErrNotExist) {
		return suite, err
	}
	versionCmd := exec.CommandContext(ctx, o.CodexBin, "--version")
	versionCmd.Env = withoutTypeSafeKey(os.Environ())
	versionBytes, err := versionCmd.Output()
	if err != nil {
		return suite, fmt.Errorf("Codex CLI unavailable: %w", err)
	}
	version := strings.TrimSpace(string(versionBytes))
	for _, id := range taskIDs {
		task, err := loadTask(o.RepoRoot, id)
		if err != nil {
			return suite, err
		}
		for _, mode := range modes {
			fmt.Fprintf(os.Stderr, "running %s / %s\n", id, mode)
			record := runOne(ctx, o, task, mode, ledger, client)
			record.CodexVersion = version
			record.CodexModel = o.CodexModel
			record.JevModel = jev.Model
			record.FinishedAt = time.Now().UTC()
			suite.Records = append(suite.Records, record)
			suite.GeneratedAt = time.Now().UTC()
			suite.Budget, err = ledger.Read()
			if err != nil {
				return suite, fmt.Errorf("read Jev ledger after run: %w", err)
			}
			if err := SaveSuite(o.OutputDir, suite); err != nil {
				return suite, err
			}
			fmt.Fprintf(os.Stderr, "completed %s / %s: %s\n", id, mode, record.Status)
		}
	}
	return suite, nil
}

func runOne(ctx context.Context, o Options, task Task, mode string, ledger *jev.Ledger, client *jev.Client) (record Record) {
	record = Record{TaskID: task.ID, Mode: mode, Status: "infra_error"}
	started := time.Now()
	defer func() { record.ElapsedMS = time.Since(started).Milliseconds() }()
	before, err := ledger.Read()
	if err != nil {
		record.Error = "cannot read Jev budget ledger"
		return record
	}
	workspace, err := os.MkdirTemp("", "jev-codex-run-*")
	if err != nil {
		record.Error = "cannot create isolated workspace"
		return record
	}
	defer os.RemoveAll(workspace)
	if err := copyTree(filepath.Join(o.RepoRoot, "fixtures", task.ID), workspace); err != nil {
		record.Error = "cannot copy fixture"
		return record
	}
	prompt := task.Prompt + "\nWork only in this fixture. Run visible tests. Do not access the network, other directories, secrets, or user repositories."
	if mode == "prefilter" {
		res, err := selector.Rank(ctx, client, workspace, task.Prompt)
		if err != nil {
			record.Error = "Jev prefilter failed: " + err.Error()
			return record
		}
		record.Selected = res.SelectedPaths
		prompt += "\nJev ranked these files as most relevant: " + strings.Join(res.SelectedPaths, ", ") + ". Treat this as a hint, not proof."
	}
	args := []string{"exec", "--ignore-user-config", "--ephemeral", "--json", "--skip-git-repo-check", "--sandbox", "workspace-write", "-m", o.CodexModel, "-C", workspace}
	if mode == "autonomous" {
		self, err := os.Executable()
		if err != nil {
			record.Error = "cannot locate MCP executable"
			return record
		}
		// Rank the immutable, authored fixture rather than Codex's mutable copy:
		// agent-written files can never become TypeSafe input through this tool.
		socket, cleanup, err := broker.Start(ctx, filepath.Join(o.RepoRoot, "fixtures", task.ID), task.Prompt, client)
		if err != nil {
			record.Error = "cannot start local Jev broker"
			return record
		}
		defer cleanup()
		mcpArgs, _ := json.Marshal([]string{"mcp", "--broker-socket", socket})
		args = append(args,
			"-c", "mcp_servers.jev.command="+strconv.Quote(self),
			"-c", "mcp_servers.jev.args="+string(mcpArgs),
			"-c", `mcp_servers.jev.enabled_tools=["rank_files"]`,
			"-c", "mcp_servers.jev.required=true",
		)
		prompt += "\nYou have an optional rank_files MCP tool backed by Jev. Use it only if file relevance is uncertain; do not call it merely to satisfy the experiment."
	}
	args = append(args, prompt)
	callCtx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()
	cmd := exec.CommandContext(callCtx, o.CodexBin, args...)
	cmd.Dir = workspace
	cmd.Env = append(withoutTypeSafeKey(os.Environ()), "GOWORK=off")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err = cmd.Run()
	usage, mcpCalls, completed := parseCodexEvents(stdout.Bytes())
	record.CodexUsage, record.MCPCalls = usage, mcpCalls
	after, ledgerErr := ledger.Read()
	if ledgerErr == nil {
		record.JevCalls = after.Calls - before.Calls
		record.JevUSD = after.SpentUSD - before.SpentUSD
	}
	if err != nil || !completed {
		if errors.Is(callCtx.Err(), context.DeadlineExceeded) {
			record.Error = "Codex timed out"
		} else {
			record.Error = "Codex failed or emitted no completed turn"
		}
		return record
	}
	if err := copyTree(filepath.Join(o.RepoRoot, "_validators", task.ID), workspace); err != nil {
		record.Error = "cannot copy hidden validator"
		return record
	}
	testCtx, testCancel := context.WithTimeout(ctx, 2*time.Minute)
	defer testCancel()
	testArgs := []string{"test", "./..."}
	if task.ID == "claim-queue" {
		testArgs = []string{"test", "-race", "./..."}
	}
	testCmd := exec.CommandContext(testCtx, o.GoBin, testArgs...)
	testCmd.Dir = workspace
	testCmd.Env = append(withoutTypeSafeKey(os.Environ()), "GOWORK=off")
	if err := testCmd.Run(); err != nil {
		if errors.Is(testCtx.Err(), context.DeadlineExceeded) {
			record.Error = "hidden validation timed out"
			return record
		}
		record.Status = "fail"
		record.Error = "hidden validation tests failed"
		return record
	}
	record.Status, record.TestsPassed = "pass", true
	return record
}

func parseCodexEvents(data []byte) (CodexUsage, int, bool) {
	var usage CodexUsage
	mcpCalls := 0
	completed := false
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		var event struct {
			Type  string     `json:"type"`
			Usage CodexUsage `json:"usage"`
			Item  struct {
				Type string `json:"type"`
			} `json:"item"`
		}
		if json.Unmarshal(scanner.Bytes(), &event) != nil {
			continue
		}
		if event.Type == "turn.completed" {
			usage = event.Usage
			completed = true
		}
		if event.Type == "item.completed" && event.Item.Type == "mcp_tool_call" {
			mcpCalls++
		}
	}
	return usage, mcpCalls, completed
}

func loadTask(repoRoot, id string) (Task, error) {
	var task Task
	data, err := os.ReadFile(filepath.Join(repoRoot, "fixtures", id, "task.json"))
	if err != nil {
		return task, err
	}
	if err := json.Unmarshal(data, &task); err != nil {
		return task, err
	}
	if task.ID != id || task.Prompt == "" {
		return task, fmt.Errorf("invalid task manifest for %s", id)
	}
	return task, nil
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0700)
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("non-regular fixture entry: %s", path)
		}
		from, err := os.Open(path)
		if err != nil {
			return err
		}
		defer from.Close()
		to, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			return err
		}
		if _, err := io.Copy(to, from); err != nil {
			to.Close()
			return err
		}
		return to.Close()
	})
}

func SaveSuite(outDir string, suite Suite) error {
	data, err := json.MarshalIndent(suite, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(outDir, "results.json"), data, 0600); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "report.md"), []byte(RenderReport(suite)), 0600)
}

func RenderReport(suite Suite) string {
	var b strings.Builder
	fmt.Fprintln(&b, "# Jev × Codex pilot results")
	fmt.Fprint(&b, "\nSmall pilot; do not generalize beyond these fixtures. Jev cost excludes Codex usage.\n\n")
	fmt.Fprintln(&b, "| Task | Arm | Status | Hidden tests | Time (s) | Codex input | Codex output | MCP calls | Jev calls | Jev USD |")
	fmt.Fprintln(&b, "|---|---|---|---:|---:|---:|---:|---:|---:|---:|")
	for _, r := range suite.Records {
		fmt.Fprintf(&b, "| %s | %s | %s | %t | %.1f | %d | %d | %d | %d | %.6f |\n", r.TaskID, r.Mode, r.Status, r.TestsPassed, float64(r.ElapsedMS)/1000, r.CodexUsage.InputTokens, r.CodexUsage.OutputTokens, r.MCPCalls, r.JevCalls, r.JevUSD)
	}
	fmt.Fprintf(&b, "\nJev ledger: spent $%.6f; reserved $%.6f; cap $%.2f. Reservations remain after uncertain network failures.\n", suite.Budget.SpentUSD, suite.Budget.ReservedUSD, suite.Budget.BudgetUSD)
	fmt.Fprintln(&b, "\nEach arm has one run per task. A passing test is the only success claim; time and tokens are descriptive, not statistically significant. A missing autonomous MCP call is reported, not counted as Jev assistance.")
	return b.String()
}

func TaskIDs() []string { return slices.Clone(taskIDs) }

func withoutTypeSafeKey(env []string) []string {
	filtered := make([]string, 0, len(env))
	for _, entry := range env {
		if !strings.HasPrefix(entry, "TYPESAFE_API_KEY=") {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}
