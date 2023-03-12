package mock

import (
	"context"

	"github.com/gosom/scrapemate"
)

type MockProvider struct {
	ChansFunc func(ctx context.Context) (<-chan scrapemate.IJob, <-chan scrapemate.IJob, <-chan scrapemate.IJob)
	PushFunc  func(ctx context.Context, job scrapemate.IJob) error
}

// Chans returns the channels to get jobs from
// we have 3 channels, one for each priority
func (m *MockProvider) Chans(ctx context.Context) (<-chan scrapemate.IJob, <-chan scrapemate.IJob, <-chan scrapemate.IJob) {
	return m.ChansFunc(ctx)
}

// Push pushes a job to the job provider
func (m *MockProvider) Push(ctx context.Context, job scrapemate.IJob) error {
	return m.PushFunc(ctx, job)
}
