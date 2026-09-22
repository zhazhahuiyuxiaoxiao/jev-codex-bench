package jev

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testRequest() Request {
	return Request{Model: Model, State: "public fixture", Questions: map[string]Question{"relevant": {Type: "noul", Instructions: "Is this relevant?"}}}
}

func TestEvaluateRecordsActualCost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer dummy" {
			t.Error("missing bearer token")
		}
		fmt.Fprint(w, `{"model":"jev-1.13.0","answers":{"relevant":{"type":"noul","noul":0.8}},"usage":{"input_tokens":100,"output_tokens":1}}`)
	}))
	defer server.Close()
	ledger := &Ledger{Path: filepath.Join(t.TempDir(), "budget.json")}
	client := &Client{HTTP: server.Client(), URL: server.URL, Key: "dummy", Ledger: ledger}
	response, err := client.Evaluate(context.Background(), testRequest())
	if err != nil {
		t.Fatal(err)
	}
	if response.Answers["relevant"].Noul != 0.8 {
		t.Fatal("unexpected answer")
	}
	state, err := ledger.Read()
	if err != nil {
		t.Fatal(err)
	}
	if state.Calls != 1 || state.ReservedUSD != 0 || math.Abs(state.SpentUSD-100*PricePerMillion/1_000_000) > 1e-12 {
		t.Fatalf("unexpected ledger: %+v", state)
	}
}

func TestEvaluateFailureKeepsReservation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "private echoed content", http.StatusBadGateway)
	}))
	defer server.Close()
	ledger := &Ledger{Path: filepath.Join(t.TempDir(), "budget.json")}
	client := &Client{HTTP: server.Client(), URL: server.URL, Key: "dummy", Ledger: ledger}
	_, err := client.Evaluate(context.Background(), testRequest())
	if err == nil || strings.Contains(err.Error(), "private echoed content") {
		t.Fatalf("unsafe error: %v", err)
	}
	state, err := ledger.Read()
	if err != nil {
		t.Fatal(err)
	}
	if state.Calls != 0 || state.SpentUSD != 0 || state.ReservedUSD <= 0 {
		t.Fatalf("unexpected ledger: %+v", state)
	}
}

func TestLedgerRejectsOverspend(t *testing.T) {
	ledger := &Ledger{Path: filepath.Join(t.TempDir(), "budget.json")}
	called := false
	err := ledger.ReserveAndExecute(0.51, func() (float64, error) { called = true; return 0, nil })
	if err == nil || called {
		t.Fatalf("expected preflight rejection, got %v", err)
	}
}

func TestMissingKeyDoesNotCreateLedger(t *testing.T) {
	path := filepath.Join(t.TempDir(), "budget.json")
	client := &Client{Key: "", Ledger: &Ledger{Path: path}}
	if _, err := client.Evaluate(context.Background(), testRequest()); err == nil {
		t.Fatal("expected missing key error")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing key should not create ledger, stat: %v", err)
	}
}

func TestLedgerReadCreatesParentDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private", "budget.json")
	state, err := (&Ledger{Path: path}).Read()
	if err != nil || state.BudgetUSD != DefaultBudgetUSD {
		t.Fatalf("state %+v, error %v", state, err)
	}
}
