package claimqueue_test

import (
	"sync"
	"testing"

	claimqueue "example.com/claim-queue"
)

func TestConcurrentClaimUnique(t *testing.T) {
	const n = 20
	ids := make([]int, n)
	for i := range ids {
		ids[i] = i + 1
	}
	q := claimqueue.New(ids...)
	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make(chan int, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if job, ok := q.Claim(); ok {
				results <- job.ID
			}
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	seen := map[int]bool{}
	for id := range results {
		if seen[id] {
			t.Errorf("job %d was claimed twice", id)
		}
		seen[id] = true
	}
	if len(seen) != n {
		t.Errorf("claimed %d distinct jobs, want %d", len(seen), n)
	}
}
