---
name: scrapemate
description: Read a web page as Markdown or HTML using the local Scrapemate CLI, including JavaScript-rendered pages that need a selector wait.
---

# Read a page with Scrapemate

Run `scrapemate fetch URL` for readable Markdown. For a long page, redirect stdout to a file and read the saved snapshot in chunks. Use `--format html` for the full HTML document and `--format raw` for the original response body, including JSON, plain text, or binary data.

If content is populated by JavaScript, use `scrapemate fetch URL --js --wait 'selector'` with a selector for the desired content. Browser mode requires the matching Playwright driver and Chromium. The [fetch guide](https://github.com/gosom/scrapemate/blob/main/docs/fetch.md) covers binary installation and browser setup.

Check the exit status. A non-2xx response exits with 1 and reports its HTTP status on stderr. The output can still contain the error page. Each command starts a fresh session. For a workflow that needs cookies across pages, use the Go `scrape.Client`.
