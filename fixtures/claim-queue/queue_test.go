package claimqueue

import "testing"

func TestSequentialClaim(t *testing.T) {
	q := New(1, 2)
	first, ok := q.Claim()
	if !ok || first.ID != 1 {
		t.Fatalf("first claim: %+v %t", first, ok)
	}
	second, ok := q.Claim()
	if !ok || second.ID != 2 {
		t.Fatalf("second claim: %+v %t", second, ok)
	}
}
