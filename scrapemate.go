package scrapemate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gosom/kit/logging"
)

// New creates a new scrapemate
func New(options ...func(*scrapeMate) error) (*scrapeMate, error) {
	s := &scrapeMate{}
	for _, opt := range options {
		if err := opt(s); err != nil {
			return nil, err
		}
	}

	if s.jobProvider == nil {
		return nil, ErrorNoJobProvider
	}
	if s.httpFetcher == nil {
		return nil, ErrorNoHttpFetcher
	}
	// here we can set default options
	s.results = make(chan Result)

	if s.ctx == nil {
		s.ctx, s.cancelFn = context.WithCancelCause(context.Background())
	}
	if s.cancelFn == nil {
		s.ctx, s.cancelFn = context.WithCancelCause(s.ctx)
	}
	if s.log == nil {
		s.log = logging.Get().With("component", "scrapemate")
		s.log.Debug("using default logger")
	}
	if s.concurrency == 0 {
		s.concurrency = 1
	}
	return s, nil
}

// WithFailed sets the failed jobs channel for the scrapemate
func WithFailed() func(*scrapeMate) error {
	return func(s *scrapeMate) error {
		s.failedJobs = make(chan IJob)
		return nil
	}
}

// WithContext sets the context for the scrapemate
func WithContext(ctx context.Context, cancelFn context.CancelCauseFunc) func(*scrapeMate) error {
	return func(s *scrapeMate) error {
		if ctx == nil {
			return ErrorNoContext
		}
		s.ctx = ctx
		s.cancelFn = cancelFn
		return nil
	}
}

// WithLogger sets the logger for the scrapemate
func WithLogger(log logging.Logger) func(*scrapeMate) error {
	return func(s *scrapeMate) error {
		if log == nil {
			return ErrorNoLogger
		}
		s.log = log
		return nil
	}
}

// WithJobProvider sets the job provider for the scrapemate
func WithJobProvider(provider JobProvider) func(*scrapeMate) error {
	return func(s *scrapeMate) error {
		if provider == nil {
			return errors.New("job provider is nil")
		}
		s.jobProvider = provider
		return nil
	}
}

// WithConcurrency sets the concurrency for the scrapemate
func WithConcurrency(concurrency int) func(*scrapeMate) error {
	return func(s *scrapeMate) error {
		if concurrency < 1 {
			return ErrorConcurrency
		}
		s.concurrency = concurrency
		return nil
	}
}

// WithHttpFetcher sets the http fetcher for the scrapemate
func WithHttpFetcher(client HttpFetcher) func(*scrapeMate) error {
	return func(s *scrapeMate) error {
		if client == nil {
			return ErrorNoHttpFetcher
		}
		s.httpFetcher = client
		return nil
	}
}

// WithHtmlParser sets the html parser for the scrapemate
func WithHtmlParser(parser HtmlParser) func(*scrapeMate) error {
	return func(s *scrapeMate) error {
		if parser == nil {
			return ErrorNoHtmlParser
		}
		s.htmlParser = parser
		return nil
	}
}

// Result is the struct items of which the Results channel has
type Result struct {
	Job  IJob
	Data any
}

// scrapemate contains unexporter fields
type scrapeMate struct {
	log         logging.Logger
	ctx         context.Context
	cancelFn    context.CancelCauseFunc
	jobProvider JobProvider
	concurrency int
	httpFetcher HttpFetcher
	htmlParser  HtmlParser
	results     chan Result
	failedJobs  chan IJob
}

// Start starts the scraper
func (s *scrapeMate) Start() error {
	s.log.Info("starting scrapemate")
	defer func() {
		close(s.results)
		if s.failedJobs != nil {
			close(s.failedJobs)
		}
		s.log.Info("scrapemate exited")
	}()
	exitChan := make(chan os.Signal, 1)
	signal.Notify(exitChan, os.Interrupt, syscall.SIGTERM)
	s.waitForSignal(exitChan)
	wg := sync.WaitGroup{}
	wg.Add(s.concurrency)
	for i := 0; i < s.concurrency; i++ {
		go func() {
			defer wg.Done()
			s.startWorker(s.ctx)
		}()
	}
	wg.Wait()
	<-s.Done()
	return s.Err()
}

// Concurrency returns how many workers are running in parallel
func (s *scrapeMate) Concurrency() int {
	return s.concurrency
}

// Results returns a channel containing the results
func (s *scrapeMate) Results() <-chan Result {
	return s.results
}

// Failed returns the chanell that contains the jobs that failed. It's nil if
// you don't use the WithFailed option
func (s *scrapeMate) Failed() <-chan IJob {
	return s.failedJobs
}

// DoJob scrapes a job and returns it's result
func (s *scrapeMate) DoJob(ctx context.Context, job IJob) (result any, next []IJob, err error) {
	startTime := time.Now().UTC()
	s.log.Debug("starting job", "job", job)
	var resp Response
	defer func() {
		args := []any{
			"job", job,
		}
		if r := recover(); r != nil {
			args = append(args, "error", r)
			args = append(args, "status", "failed")
			err = fmt.Errorf("panic while executing job: %v", r)
			return
		}
		if resp.Error != nil {
			args = append(args, "error", resp.Error)
			args = append(args, "status", "failed")
		} else {
			args = append(args, "status", "success")
		}
		args = append(args, "duration", time.Now().UTC().Sub(startTime))
		s.log.Info("job finished", args...)
	}()

	resp = s.doFetch(ctx, job)
	if resp.Error != nil {
		err = resp.Error
		return
	}

	// cache the response if needed
	// process the response
	if s.htmlParser != nil {
		resp.Document, err = s.htmlParser.Parse(ctx, resp.Body)
		if err != nil {
			s.log.Error("error while setting document", "error", err)
			return
		}
	}
	ctx = context.WithValue(ctx, "log", s.log.With("jobid", job.GetID()))
	result, next, err = job.Process(ctx)
	if err != nil {
		// TODO shall I retry?
		s.log.Error("error while processing job", "error", err)
		return
	}
	return
}

func (s *scrapeMate) doFetch(ctx context.Context, job IJob) (ans Response) {
	var ok bool
	defer func() {
		if !ok && ans.Error == nil {
			ans.Error = fmt.Errorf("status code %d", ans.StatusCode)
		}
	}()
	maxRetries := s.getMaxRetries(job)
	delay := time.Millisecond * 100
	retryPolicy := job.GetRetryPolicy()
	retry := 0
	for {
		ans = s.httpFetcher.Fetch(ctx, job)
		ok = job.DoCheckResponse(ans)
		if ok {
			return
		}

		if retryPolicy == DiscardJob {
			s.log.Warn("discarding job because of policy")
			return
		}

		if retryPolicy == StopScraping {
			s.log.Warn("stopping scraping because of policy")
			s.cancelFn(errors.New("stopping scraping because of policy"))
			return
		}

		if retry >= maxRetries {
			return
		}
		retry++
		switch retryPolicy {
		case RetryJob:
			time.Sleep(delay)
			if delay > job.GetMaxRetryDelay() {
				delay = job.GetMaxRetryDelay()
			} else {
				delay = delay * 2
			}
		case RefreshIP:
			// TODO
		}
	}
}

func (s *scrapeMate) getMaxRetries(job IJob) int {
	maxRetries := job.GetMaxRetries()
	if maxRetries > 5 {
		maxRetries = 5
	}
	return maxRetries
}

// Done returns a channel  that's closed when the work is done
func (s *scrapeMate) Done() <-chan struct{} {
	return s.ctx.Done()
}

// Err returns the error that caused scrapemate's context cancellation
func (s *scrapeMate) Err() error {
	return context.Cause(s.ctx)
}

func (s *scrapeMate) waitForSignal(sigChan <-chan os.Signal) {
	go func() {
		select {
		case <-sigChan:
			s.log.Info("received signal, shutting down")
			s.cancelFn(errors.New("received signal"))
		}
	}()
}

func (s *scrapeMate) startWorker(ctx context.Context) {
	jobc, errc := s.jobProvider.Jobs(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case err := <-errc:
			s.log.Error("error while getting jobs...going to wait a bit", "error", err)
			time.Sleep(1 * time.Second)
			jobc, errc = s.jobProvider.Jobs(ctx)
			s.log.Info("restarted job provider")
		case job := <-jobc:
			ans, next, err := s.DoJob(ctx, job)
			if err != nil {
				s.log.Error("error while processing job", "error", err)
				s.pushToFailedJobs(job)
				continue
			}
			if err := s.finishJob(ctx, job, ans, next); err != nil {
				s.log.Error("error while finishing job", "error", err)
				s.pushToFailedJobs(job)
			}
		}
	}
}

func (s *scrapeMate) pushToFailedJobs(job IJob) {
	if s.failedJobs != nil {
		s.failedJobs <- job
	}
}

func (s *scrapeMate) finishJob(ctx context.Context, job IJob, ans any, next []IJob) error {
	if err := s.pushJobs(ctx, next); err != nil {
		return fmt.Errorf("%w: while pushing jobs", err)
	}
	s.results <- Result{
		Job:  job,
		Data: ans,
	}
	return nil
}

func (s *scrapeMate) pushJobs(ctx context.Context, jobs []IJob) error {
	for i := range jobs {
		fmt.Println("pushing job", jobs[i])
		if err := s.jobProvider.Push(ctx, jobs[i]); err != nil {
			return err
		}
	}
	return nil
}
