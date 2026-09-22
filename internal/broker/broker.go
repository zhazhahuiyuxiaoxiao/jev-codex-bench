package broker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	"jev-codex-bench/internal/selector"
)

type request struct{}

type response struct {
	Result selector.Result `json:"result"`
	Error  string          `json:"error,omitempty"`
}

// Start creates a private local socket. The broker, not Codex or its MCP
// subprocess, owns the TypeSafe key and the fixed public workspace.
func Start(ctx context.Context, workspace, fixedTask string, evaluator selector.Evaluator) (string, func(), error) {
	dir, err := os.MkdirTemp("", "jev-broker-*")
	if err != nil {
		return "", nil, err
	}
	if err := os.Chmod(dir, 0700); err != nil {
		os.RemoveAll(dir)
		return "", nil, err
	}
	socket := filepath.Join(dir, "rank.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		os.RemoveAll(dir)
		return "", nil, err
	}
	if err := os.Chmod(socket, 0600); err != nil {
		listener.Close()
		os.RemoveAll(dir)
		return "", nil, err
	}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go serveOne(ctx, conn, workspace, fixedTask, evaluator)
		}
	}()
	cleanup := func() { listener.Close(); os.RemoveAll(dir) }
	return socket, cleanup, nil
}

func serveOne(ctx context.Context, conn net.Conn, workspace, fixedTask string, evaluator selector.Evaluator) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(40 * time.Second))
	var req request
	if err := json.NewDecoder(io.LimitReader(conn, 4096)).Decode(&req); err != nil {
		return
	}
	result, err := selector.Rank(ctx, evaluator, workspace, fixedTask)
	resp := response{Result: result}
	if err != nil {
		resp.Error = err.Error()
	}
	_ = json.NewEncoder(conn).Encode(resp)
}

func Rank(ctx context.Context, socket string) (selector.Result, error) {
	var zero selector.Result
	if socket == "" {
		return zero, errors.New("broker socket is required")
	}
	dialer := net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(ctx, "unix", socket)
	if err != nil {
		return zero, fmt.Errorf("connect to local Jev broker: %w", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(45 * time.Second))
	if err := json.NewEncoder(conn).Encode(request{}); err != nil {
		return zero, err
	}
	var resp response
	if err := json.NewDecoder(io.LimitReader(conn, 64*1024)).Decode(&resp); err != nil {
		return zero, err
	}
	if resp.Error != "" {
		return zero, errors.New(resp.Error)
	}
	return resp.Result, nil
}
