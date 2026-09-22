package cachettl_test

import (
	"testing"
	"time"

	cachettl "example.com/cache-ttl"
)

func TestExpiryBoundary(t *testing.T) {
	now := time.Unix(0, 0)
	c := cachettl.New(func() time.Time { return now })
	c.Set("a", "value", time.Second)
	now = now.Add(time.Second)
	if value, ok := c.Get("a"); ok || value != "" {
		t.Fatalf("expired entry returned %q, %t", value, ok)
	}
}

func TestNonPositiveTTL(t *testing.T) {
	now := time.Unix(0, 0)
	c := cachettl.New(func() time.Time { return now })
	for _, ttl := range []time.Duration{0, -time.Second} {
		c.Set("a", "value", ttl)
		if value, ok := c.Get("a"); ok || value != "" {
			t.Fatalf("ttl %s returned %q, %t", ttl, value, ok)
		}
	}
}
