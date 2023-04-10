package scrapemate

import "errors"

var (
	// ErrorNoJobProvider returned when you do not set a job provider in initialization
	ErrorNoJobProvider = errors.New("no job provider set")
	// ErroExitSignal is returned when scrapemate exits because of a system interrupt
	ErrorExitSignal = errors.New("exit signal received")
	// ErrorNoLogger returned when you try to initialize it with a nil logger
	ErrorNoLogger = errors.New("no logger set")
	// ErrorNoContext returned when you try to initialized it with a nil context
	ErrorNoContext = errors.New("no context set")
	// ErrorConcurrency returned when you try to initialize it with concurrency <1
	ErrorConcurrency = errors.New("concurrency must be greater than 0")
	// ErrorNoHttpFetcher returned when you try to initialize with a nil httpFetcher
	ErrorNoHttpFetcher = errors.New("no http fetcher set")
	// ErrorNoHtmlParser returned when you try to initialized with a nil HtmlParser
	ErrorNoHtmlParser = errors.New("no html parser set")
	// ErrorNoCacher returned when you try to initialized with a nil Cacher
	ErrorNoCacher = errors.New("no cacher set")
)
