package csvexport

import "testing"

func TestOrdinaryCell(t *testing.T) {
	got, err := EncodeRow([]string{"Alice", "123"})
	if err != nil || got != "Alice,123\n" {
		t.Fatalf("got %q, %v", got, err)
	}
}
