# Fetch a page

From a checkout, build the CLI and read a page. HTTP mode works without an API key or a browser.

```sh
go build -o scrapemate ./cmd/scrapemate
./scrapemate fetch https://example.com --format markdown
```

## Install a release binary

On the [releases page](https://github.com/gosom/scrapemate/releases), choose a release containing CLI assets. Download the file for your operating system and CPU. The binary runs without a Go installation.

| Platform | Intel / AMD 64-bit | ARM 64-bit |
| --- | --- | --- |
| Linux | `scrapemate-linux-amd64` | `scrapemate-linux-arm64` |
| macOS | `scrapemate-darwin-amd64` | `scrapemate-darwin-arm64` (Apple Silicon) |
| Windows | `scrapemate-windows-amd64.exe` | `scrapemate-windows-arm64.exe` |

The release includes `SHA256SUMS` for verifying downloads. On Linux or macOS, rename the file to `scrapemate`, run `chmod +x scrapemate`, then run `./scrapemate --help`. On Windows, rename it to `scrapemate.exe` and run `.\scrapemate.exe --help`. Put the executable on your `PATH` to call it from any directory.

For example, from a Linux x86-64 terminal with the GitHub CLI, download the binary from the latest release with CLI assets once it is published.

```sh
gh release download --repo gosom/scrapemate --pattern scrapemate-linux-amd64
chmod +x scrapemate-linux-amd64
./scrapemate-linux-amd64 fetch https://example.com
```

## Read and save output

```sh
scrapemate fetch https://example.com
scrapemate fetch https://example.com --format html
scrapemate fetch https://example.com --timeout 10s > page.md
```

Markdown is the default. It uses the main landmark, a lone article, or the page body, and omits navigation and footers. Relative links resolve against the final URL and any HTML base URL. Use `--format html` for the full document. To read a long page in pieces, save it once and read ranges from that file.

Page content goes to stdout. Errors go to stderr. Exit code 0 means success, 1 means a fetch, conversion or output failure, and 2 means a command or flag usage error. A non-2xx status returns 1 and prints the status to stderr. Its page content is still written when the selected format is available.

## Render a JavaScript page

The browser mode needs the Playwright driver and Chromium version matched to Scrapemate. From the source checkout for your version, run the following setup once. The version in its `go.mod` selects the driver.

```sh
go run github.com/mxschmitt/playwright-go/cmd/playwright install --with-deps chromium
scrapemate fetch https://quotes.toscrape.com/js/ --js --wait .quote
```

Each CLI invocation owns a fresh session.

## Use an authenticated proxy

Set `SCRAPEMATE_PROXY` in your shell for both HTTP and browser fetches. Replace the example credentials, host, and port with your provider’s values. A trailing `/` is optional, so you can copy the proxy URL directly from your provider’s curl example.

```sh
export SCRAPEMATE_PROXY='http://USERNAME:PASSWORD@proxy.example:8080/'
scrapemate fetch https://example.com
```

From the repository root, you can run the same command with `go run ./cmd/scrapemate fetch https://example.com`.

To verify the outgoing IP, keep the variable set and run the bundled Go example from the repository root. It writes the plain-text IP to stdout and the HTTP status to stderr. Compare the IP with the exit IP listed by your proxy provider.

```sh
go run ./examples/scrape-client https://ipv4.webshare.io/
```

Run `unset SCRAPEMATE_PROXY` when you’re done using the proxy.

## Fetch from Go

```go
client, err := scrape.New()
if err != nil {
    return err
}
defer client.Close()

page, err := client.Fetch(ctx, target, scrape.WithOutputs(scrape.Markdown()))
if err != nil {
    return err
}
doc, _ := page.Artifact("markdown")
_, err = os.Stdout.Write(doc.Data)
return err
```

Import `github.com/gosom/scrapemate/scrape`. The original response and HTML snapshot remain available on `page`. Reuse the client to preserve cookies across fetches. A non-2xx response is inspectable through `page.Response.StatusCode`. A conversion failure returns the available response and an error.

Run the Go example from the repository root.

```sh
go run ./examples/scrape-client -markdown https://example.com
```
