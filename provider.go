package scrapemate

import "context"

// JobProvider is an interface for job providers
// a job provider is a service that provides jobs to scrapemate
// scrapemate will call the job provider to get jobs
type JobProvider interface {
	Jobs(ctx context.Context) (<-chan IJob, <-chan error)
	// Push pushes a job to the job provider
	Push(ctx context.Context, job IJob) error
}
