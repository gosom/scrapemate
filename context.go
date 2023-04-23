package scrapemate

import (
	"context"

	"github.com/gosom/kit/logging"
)

// GetLoggerFromContext returns a logger from the context.
func GetLoggerFromContext(ctx context.Context) logging.Logger {
	log := ctx.Value("log").(logging.Logger)
	return log
}
