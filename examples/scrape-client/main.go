// Command scrape-client fetches one page and writes its body to stdout.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/gosom/scrapemate/scrape"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() (err error) {
	useJS := flag.Bool("js", false, "render the page with Chromium")
	markdown := flag.Bool("markdown", false, "write the page's main content as Markdown")
	selector := flag.String("wait", "", "wait for a visible CSS selector in Chromium")
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), "Usage: scrape-client [flags] URL")
		flag.PrintDefaults()
	}

	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		return errors.New("expected one URL")
	}

	var options []scrape.Option
	if *useJS {
		options = append(options, scrape.WithJS())
	}

	if proxy := os.Getenv("SCRAPEMATE_PROXY"); proxy != "" {
		options = append(options, scrape.WithProxies([]string{proxy}))
	}

	client, err := scrape.New(options...)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, client.Close()) }()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	var fetchOptions []scrape.FetchOption
	if *markdown {
		fetchOptions = append(fetchOptions, scrape.WithOutputs(scrape.Markdown()))
	}

	if *selector != "" {
		fetchOptions = append(fetchOptions, scrape.WaitFor(*selector))
	}

	result, err := client.Fetch(ctx, flag.Arg(0), fetchOptions...)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "%d %s\n", result.Response.StatusCode, result.Response.URL)

	if *markdown {
		doc, ok := result.Artifact("markdown")
		if !ok {
			return errors.New("missing Markdown artifact")
		}

		_, err = os.Stdout.Write(doc.Data)

		return err
	}

	if html := result.HTML(); html != "" {
		_, err = fmt.Fprint(os.Stdout, html)
	} else {
		_, err = os.Stdout.Write(result.Response.Body)
	}

	return err
}
