package claimqueue

func (q *Queue) Size() int { return len(q.jobs) }
