package scrapemateapp

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gosom/scrapemate"
	"golang.org/x/sync/errgroup"

	"github.com/gosom/scrapemate/adapters/cache/filecache"
	"github.com/gosom/scrapemate/adapters/cache/leveldbcache"
	jsfetcher "github.com/gosom/scrapemate/adapters/fetchers/jshttp"
	fetcher "github.com/gosom/scrapemate/adapters/fetchers/nethttp"
	parser "github.com/gosom/scrapemate/adapters/parsers/goqueryparser"
	memprovider "github.com/gosom/scrapemate/adapters/providers/memory"
)

type ScrapemateApp struct {
	cfg *config

	ctx    context.Context
	cancel context.CancelCauseFunc

	provider scrapemate.JobProvider
	cacher   scrapemate.Cacher
}

// NewScrapemateApp creates a new ScrapemateApp.
func NewScrapeMateApp(cfg *config) (*ScrapemateApp, error) {
	app := ScrapemateApp{
		cfg: cfg,
	}

	return &app, nil
}

// Start starts the app.
func (app *ScrapemateApp) Start(ctx context.Context, seedJobs ...scrapemate.IJob) error {
	g, ctx := errgroup.WithContext(ctx)
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(errors.New("closing app"))

	mate, err := app.getMate(ctx)
	if err != nil {
		return err
	}
	defer app.Close()

	for i := range app.cfg.Writers {
		writer := app.cfg.Writers[i]
		g.Go(func() error {
			if err := writer.Run(ctx, mate.Results()); err != nil {
				cancel(err)
				return err
			}
			return nil
		})
	}

	g.Go(func() error {
		return mate.Start()
	})

	g.Go(func() error {
		for i := range seedJobs {
			if err := app.provider.Push(ctx, seedJobs[i]); err != nil {
				return err
			}
		}
		return nil
	})
	return g.Wait()
}

// Close closes the app.
func (app *ScrapemateApp) Close() error {
	if app.cacher != nil {
		app.cacher.Close()
	}
	return nil
}

func (app *ScrapemateApp) getMate(ctx context.Context) (*scrapemate.ScrapeMate, error) {
	var err error
	app.provider, err = app.getProvider()
	if err != nil {
		return nil, err
	}
	fetcher, err := app.getFetcher()
	if err != nil {
		return nil, err
	}
	app.cacher, err = app.getCacher()
	if err != nil {
		return nil, err
	}
	params := []func(*scrapemate.ScrapeMate) error{
		scrapemate.WithContext(ctx, app.cancel),
		scrapemate.WithJobProvider(app.provider),
		scrapemate.WithHttpFetcher(fetcher),
		scrapemate.WithHtmlParser(parser.New()),
		scrapemate.WithConcurrency(app.cfg.Concurrency),
	}
	if app.cacher != nil {
		params = append(params, scrapemate.WithCache(app.cacher))
	}
	mate, err := scrapemate.New(params...)
	return mate, err
}

func (app *ScrapemateApp) getCacher() (scrapemate.Cacher, error) {
	var (
		cacher scrapemate.Cacher
		err    error
	)
	switch app.cfg.CacheType {
	case "file":
		cacher, err = filecache.NewFileCache(app.cfg.CachePath)
	case "leveldb":
		cacher, err = leveldbcache.NewLevelDBCache(app.cfg.CachePath)
	}
	return cacher, err
}

func (app *ScrapemateApp) getFetcher() (scrapemate.HttpFetcher, error) {
	var httpFetcher scrapemate.HttpFetcher
	var err error
	switch app.cfg.UseJS {
	case true:
		httpFetcher, err = jsfetcher.New(true)
		if err != nil {
			return nil, err
		}
	default:
		httpFetcher = fetcher.New(&http.Client{
			Timeout: 10 * time.Second,
		})
	}
	return httpFetcher, nil
}

func (app *ScrapemateApp) getProvider() (scrapemate.JobProvider, error) {
	var provider scrapemate.JobProvider
	switch app.cfg.Provider {
	case nil:
		provider = memprovider.New()
	default:
		provider = app.cfg.Provider
	}
	return provider, nil
}
