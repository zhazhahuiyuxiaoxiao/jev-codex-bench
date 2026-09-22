package claimqueue

import "time"

type Job struct {
	ID     int
	Status string
}

type Queue struct {
	jobs []*Job
}

func New(ids ...int) *Queue {
	q := &Queue{}
	for _, id := range ids {
		q.jobs = append(q.jobs, &Job{ID: id, Status: "pending"})
	}
	return q
}

func (q *Queue) Claim() (Job, bool) {
	for _, job := range q.jobs {
		if job.Status != "pending" {
			continue
		}
		// Simulate a slow claim operation. Two callers can observe the same job.
		time.Sleep(time.Millisecond)
		job.Status = "working"
		return *job, true
	}
	return Job{}, false
}
