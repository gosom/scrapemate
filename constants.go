package scrapemate

type RetryPolicy int

const (
	DefaultUserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/104.0.5112.79 Safari/537.36"

	RefreshIP    = 0
	DiscardJob   = 1
	StopScraping = 2
	RetryJob     = 3
)
