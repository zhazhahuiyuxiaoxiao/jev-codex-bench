package jev

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

const DefaultBudgetUSD = 0.50

type LedgerState struct {
	BudgetUSD   float64 `json:"budget_usd"`
	SpentUSD    float64 `json:"spent_usd"`
	ReservedUSD float64 `json:"reserved_usd"`
	Calls       int     `json:"calls"`
}

type Ledger struct {
	Path string
}

func (l *Ledger) Read() (LedgerState, error) {
	if l == nil || l.Path == "" {
		return LedgerState{}, errors.New("ledger path is required")
	}
	if err := os.MkdirAll(filepath.Dir(l.Path), 0700); err != nil {
		return LedgerState{}, err
	}
	f, err := os.OpenFile(l.Path, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return LedgerState{}, err
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_SH); err != nil {
		return LedgerState{}, err
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return decodeLedger(f)
}

// ReserveAndExecute holds the ledger lock over the network request. This keeps
// separate bench and MCP processes from racing the same spending limit.
func (l *Ledger) ReserveAndExecute(reserved float64, call func() (float64, error)) error {
	if l == nil || l.Path == "" || reserved <= 0 {
		return errors.New("valid ledger and reservation are required")
	}
	if err := os.MkdirAll(filepath.Dir(l.Path), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(l.Path, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	state, err := decodeLedger(f)
	if err != nil {
		return err
	}
	if state.SpentUSD+state.ReservedUSD+reserved > state.BudgetUSD {
		return fmt.Errorf("Jev budget exceeded: %.6f of %.2f USD used or reserved", state.SpentUSD+state.ReservedUSD, state.BudgetUSD)
	}
	state.ReservedUSD += reserved
	if err := writeLedger(f, state); err != nil {
		return err
	}
	actual, callErr := call()
	if callErr != nil {
		// The request might have reached TypeSafe. Keep the reservation.
		return callErr
	}
	if actual < 0 || actual > reserved {
		return fmt.Errorf("actual Jev charge %.6f exceeded reservation %.6f; reservation retained", actual, reserved)
	}
	state.ReservedUSD -= reserved
	state.SpentUSD += actual
	state.Calls++
	return writeLedger(f, state)
}

func decodeLedger(f *os.File) (LedgerState, error) {
	if _, err := f.Seek(0, 0); err != nil {
		return LedgerState{}, err
	}
	var state LedgerState
	err := json.NewDecoder(f).Decode(&state)
	if err != nil {
		// An empty, newly created file starts with the fixed budget.
		if info, statErr := f.Stat(); statErr == nil && info.Size() == 0 {
			return LedgerState{BudgetUSD: DefaultBudgetUSD}, nil
		}
		return state, fmt.Errorf("invalid ledger: %w", err)
	}
	if state.BudgetUSD != DefaultBudgetUSD || state.SpentUSD < 0 || state.ReservedUSD < 0 || state.Calls < 0 {
		return state, errors.New("ledger has invalid budget or counters")
	}
	return state, nil
}

func writeLedger(f *os.File, state LedgerState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := f.Truncate(0); err != nil {
		return err
	}
	if _, err := f.Seek(0, 0); err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		return err
	}
	return f.Sync()
}
