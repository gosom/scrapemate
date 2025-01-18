## Javascript rendering

Many websites require Javascript to run.
Scrapemate has builtin support to scrape websites that require Javascript.

In order to use a headless browser add `scrapemateapp.WithJS()` option as an argument
in `scrapemateapp.NewConfig` function

Example:

```go
	cfg, err := scrapemateapp.NewConfig(
		writers,
		scrapemateapp.WithConcurrency(1),
		scrapemateapp.WithJS(),
		scrapemateapp.WithExitOnInactivity(time.Minute)
	)
```

the `WithJs` function can also accept the following options:

```go
scrapematepp.WithJS(
	scrapemateapp.Headfull(),
	scrapemateapp.DisableImages(),
)
```

- `scrapemateapp.Headfull()` makes the browser run in non headless mode.
This is useful for debugging

- `scrapemateapp.DisableImages()` does not load images and it can help makes your
scraper faster

When using `scrapemateapp.WithJS` option you can control the browser by overriding 

`BrowserActions(ctx context.Context, page playwright.Page) scrapemate.Response`

method of scrapemate.Job .

Find an example in [google maps scraper project](https://github.com/gosom/google-maps-scraper/blob/main/gmaps/job.go#L140)

