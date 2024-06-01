package scrapemateapp

import (
	"errors"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gosom/scrapemate"
)

// NewConfig creates a new config with default values.
func NewConfig(writers []scrapemate.ResultWriter, options ...func(*Config) error) (*Config, error) {
	cfg := Config{
		Writers: writers,
	}
	// defaults
	cfg.Concurrency = DefaultConcurrency
	for _, opt := range options {
		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// WithConcurrency sets the concurrency of the app.
func WithConcurrency(concurrency int) func(*Config) error {
	return func(o *Config) error {
		o.Concurrency = concurrency

		return o.validate()
	}
}

// WithCache sets the cache type and path of the app.
func WithCache(cacheType, cachePath string) func(*Config) error {
	return func(o *Config) error {
		o.CacheType = cacheType
		o.CachePath = cachePath

		return o.validate()
	}
}

// WithJS sets the app to use JavaScript to render the pages.
func WithJS(opts ...func(*jsOptions)) func(*Config) error {
	return func(o *Config) error {
		o.UseJS = true

		for _, opt := range opts {
			opt(&o.JSOpts)
		}

		return o.validate()
	}
}

// WithProvider sets the provider of the app.
func WithProvider(provider scrapemate.JobProvider) func(*Config) error {
	return func(o *Config) error {
		if provider == nil {
			return errors.New("provider cannot be nil")
		}

		o.Provider = provider

		return nil
	}
}

// WithInitJob sets the initial job of the app.
func WithInitJob(job scrapemate.IJob) func(*Config) error {
	return func(o *Config) error {
		o.InitJob = job

		return nil
	}
}

// Headfull is a helper function to create a headfull browser.
// Use it as a parameter to WithJS.
func Headfull() func(*jsOptions) {
	return func(o *jsOptions) {
		o.Headfull = true
	}
}

func DisableImages() func(*jsOptions) {
	return func(o *jsOptions) {
		o.DisableImages = true
	}
}

func Firefox() func(*jsOptions) {
	return func(o *jsOptions) {
		o.Firefox = true
	}
}

// WithExitOnInactivity sets the duration after which the app will exit if there are no more jobs to run.
func WithExitOnInactivity(duration time.Duration) func(*Config) error {
	return func(o *Config) error {
		o.ExitOnInactivityDuration = duration

		return nil
	}
}

type jsOptions struct {
	// Headfull is a flag to run the browser in headfull mode.
	// By default, the browser is run in headless mode.
	Headfull      bool
	DisableImages bool
	Firefox       bool
}

type Config struct {
	// Concurrency is the number of concurrent scrapers to run.
	// If not set, it defaults to 1.
	Concurrency int `validate:"required,gte=1"`

	// Cache is the cache to use for storing scraped data.
	// If left empty then no caching will be used.
	// Otherwise the CacheType must be one of file or leveldb.
	CacheType string `validate:"omitempty,oneof=file leveldb"`
	// CachePath is the path to the cache file or directory.
	// It is required to be a valid path if CacheType is set.
	CachePath string `validate:"required_with=CacheType"`

	// UseJS is whether to use JavaScript to render the page.
	UseJS bool `validate:"omitempty"`
	// JSOpts are the options for the JavaScript renderer.
	JSOpts jsOptions

	// ProviderType is the type of provider to use.
	// It is required to be a valid type if Provider is set.
	// If not set the memory provider will be used.
	Provider scrapemate.JobProvider

	// Writers are the writers to use for writing the results.
	// At least one writer must be provided.
	Writers []scrapemate.ResultWriter `validate:"required,gt=0"`
	// InitJob is the job to initialize the app with.
	InitJob scrapemate.IJob
	// ExitOnInactivityDuration is whether to exit the app when there are no more jobs to run.
	ExitOnInactivityDuration time.Duration
}

func (o *Config) validate() error {
	once.Do(func() {
		validate = validator.New()
	})

	return validate.Struct(o)
}
