package scrapemate

import "context"

type JobWriter interface {
	Write(ctx context.Context, data any) error
}
