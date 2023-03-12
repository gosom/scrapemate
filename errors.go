package scrapemate

import "errors"

var (
	ErrorNoJobProvider = errors.New("no job provider set")
	ErrorExitSignal    = errors.New("exit signal received")
	ErrorNoLogger      = errors.New("no logger set")
	ErrorNoContext     = errors.New("no context set")
	ErrorConcurrency   = errors.New("concurrency must be greater than 0")
	ErrorNoHttpClient  = errors.New("no http client set")
)
