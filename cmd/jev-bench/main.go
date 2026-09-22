package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"jev-codex-bench/internal/bench"
	"jev-codex-bench/internal/broker"
	"jev-codex-bench/internal/selector"
	"jev-codex-bench/internal/server"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "check-fixtures":
		fs := flag.NewFlagSet("check-fixtures", flag.ExitOnError)
		root := fs.String("root", ".", "repository root")
		fs.Parse(os.Args[2:])
		err = bench.CheckFixtures(context.Background(), *root, "go")
		if err == nil {
			fmt.Println("All three fixtures fail their hidden tests before repair, as expected.")
		}
	case "run-all":
		fs := flag.NewFlagSet("run-all", flag.ExitOnError)
		root := fs.String("root", ".", "repository root")
		out := fs.String("out", "results", "private result directory")
		model := fs.String("codex-model", "", "Codex model ID; required")
		fs.Parse(os.Args[2:])
		absRoot, absErr := filepath.Abs(*root)
		if absErr != nil {
			err = absErr
			break
		}
		absOut, absErr := filepath.Abs(*out)
		if absErr != nil {
			err = absErr
			break
		}
		suite, runErr := bench.RunAll(context.Background(), bench.Options{
			RepoRoot: absRoot, OutputDir: absOut, BudgetFile: filepath.Join(absRoot, ".local", "jev-budget.json"), CodexModel: *model,
		})
		err = runErr
		if err == nil {
			fmt.Printf("Recorded %d runs in %s\n", len(suite.Records), absOut)
		}
	case "mcp":
		fs := flag.NewFlagSet("mcp", flag.ExitOnError)
		socket := fs.String("broker-socket", "", "private local Jev broker socket")
		fs.Parse(os.Args[2:])
		if *socket == "" {
			err = fmt.Errorf("--broker-socket is required")
			break
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		err = server.Run(ctx, func(ctx context.Context) (selector.Result, error) {
			return broker.Rank(ctx, *socket)
		})
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: jev-bench check-fixtures | run-all --codex-model MODEL | mcp --broker-socket SOCKET")
}
