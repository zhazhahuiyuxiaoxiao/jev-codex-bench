package cachettl

import (
	"testing"
	"time"
)

func TestBeforeExpiry(t *testing.T) {
	now := time.Unix(0, 0)
	c := New(func() time.Time { return now })
	c.Set("a", "value", time.Second)
	if got, ok := c.Get("a"); !ok || got != "value" {
		t.Fatalf("got %q, %t", got, ok)
	}
}
