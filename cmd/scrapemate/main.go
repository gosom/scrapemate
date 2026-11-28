// Command scrapemate fetches a web page for use in a terminal or pipeline.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/gosom/scrapemate/scrape"
	"github.com/spf13/pflag"
)

const (
	usage          = "Usage: scrapemate fetch URL [--format markdown|html] [--js] [--wait SELECTOR] [--timeout DURATION]"
	formatMarkdown = "markdown"
	formatHTML     = "html"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr)

	cancel()
	os.Exit(code)
}

type fetchCommand struct {
	format  string
	js      bool
	wait    string
	timeout time.Duration
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage)

		return 2
	}

	if args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Fprintln(stdout, usage)

		return 0
	}

	if args[0] != "fetch" {
		fmt.Fprintf(stderr, "unknown command %q\n%s\n", args[0], usage)

		return 2
	}

	var cmd fetchCommand

	flags := pflag.NewFlagSet("fetch", pflag.ContinueOnError)
	flags.SetOutput(stdout)
	flags.Usage = func() {
		fmt.Fprintln(stdout, usage)
		flags.PrintDefaults()
	}
	flags.StringVar(&cmd.format, "format", formatMarkdown, "output format (markdown or html)")
	flags.BoolVar(&cmd.js, "js", false, "render with an installed Playwright Chromium browser")
	flags.StringVar(&cmd.wait, "wait", "", "wait for a visible CSS selector with --js")
	flags.DurationVar(&cmd.timeout, "timeout", 30*time.Second, "time budget for fetching the page")

	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return 0
		}

		fmt.Fprintln(stderr, err)

		return 2
	}

	if err := cmd.validate(flags); err != nil {
		fmt.Fprintln(stderr, err)

		return 2
	}

	if err := cmd.fetch(ctx, flags.Arg(0), stdout); err != nil {
		fmt.Fprintln(stderr, err)

		return 1
	}

	return 0
}

func (cmd fetchCommand) validate(flags *pflag.FlagSet) error {
	switch {
	case flags.NArg() != 1:
		return fmt.Errorf("expected one URL\n%s", usage)
	case cmd.format != formatMarkdown && cmd.format != formatHTML:
		return fmt.Errorf("unknown format %q (use markdown or html)", cmd.format)
	case cmd.timeout <= 0:
		return errors.New("--timeout must be positive")
	case flags.Changed("wait") && (!cmd.js || strings.TrimSpace(cmd.wait) == ""):
		return errors.New("--wait requires --js and a CSS selector")
	default:
		return nil
	}
}

func (cmd fetchCommand) fetch(ctx context.Context, target string, stdout io.Writer) (err error) {
	options := []scrape.Option{scrape.WithTimeout(cmd.timeout)}
	if cmd.js {
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

	var outputs []scrape.FetchOption
	if cmd.format == formatMarkdown {
		outputs = append(outputs, scrape.WithOutputs(scrape.Markdown()))
	}

	if cmd.wait != "" {
		outputs = append(outputs, scrape.WaitFor(cmd.wait))
	}

	page, err := client.Fetch(ctx, target, outputs...)
	if page == nil {
		return err
	}

	var statusErr error
	if status := page.Response.StatusCode; status < http.StatusOK || status >= http.StatusMultipleChoices {
		statusErr = fmt.Errorf("HTTP %d %s", status, page.Response.URL)
	}

	if err == nil {
		err = cmd.writePage(stdout, page)
	}

	return errors.Join(statusErr, err)
}

func (cmd fetchCommand) writePage(stdout io.Writer, page *scrape.Result) error {
	if cmd.format == formatMarkdown {
		doc, ok := page.Artifact(formatMarkdown)
		if !ok {
			return errors.New("missing Markdown artifact")
		}

		_, err := stdout.Write(doc.Data)

		return err
	}

	if !page.IsHTML() {
		return errors.New("html output requires an HTML response")
	}

	_, err := io.WriteString(stdout, page.HTML())

	return err
}
