# Quotes to Scrape request hooks example

This example shows how to use `scrapemate.RequestHookProvider` with the
Playwright JavaScript fetcher.

It opens the ToScrape infinite-scroll quotes page:

```text
https://quotes.toscrape.com/scroll
```

The page loads quote data from:

```text
https://quotes.toscrape.com/api/quotes?page=...
```

The job registers both hooks before navigation:

- `OnRequest` captures outgoing `/api/quotes` request URLs and selected request
  headers.
- `OnResponse` captures matching response URLs, status codes, and selected
  response headers.

The example prints a JSON document with the visible quote count and the captured
request/response events.

## Run

```sh
go run .
```

The first run may install Playwright browsers if they are not already available.
