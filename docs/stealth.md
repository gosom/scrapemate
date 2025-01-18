## Stealth mode

There are cases which you have to scrape a website that detects browsers
using their fingerprint. Headless browser is an option there but considered
the perfomance penalty if you do not need JS rendering the stealth mode is
something you should consider.

Stealth mode mimics a real browser's fingerprint and under the hood uses an
[HTTP client](https://github.com/Noooste/azuretls-client) that mimics real
browser's fingerprints.

### how to use

Add the option `scrapemateapp.WithStealth` 

example:

```go
cfg, err := scrapemateapp.NewConfig(
	writers,
	scrapemateapp.WithConcurrency(1),
	scrapemateapp.WithStealth("firefox"),
	scrapemateapp.WithExitOnInactivity(time.Minute)
)
```

Available browser strings are:

	"chrome"
	"firefox"
	"opera"
	"safari"
	"edge"
	"ios"


