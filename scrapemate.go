package scrapemate

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
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
	if s.ctx == nil {
		s.ctx, s.cancelFn = context.WithCancelCause(context.Background())
	}
	if s.cancelFn == nil {
		s.ctx, s.cancelFn = context.WithCancelCause(s.ctx)
	}
	// here we can set default options
	if s.log == nil {
		s.log = logging.Get().With("component", "scrapemate")
		s.log.Debug("using default logger")
	}
	if s.jobProvider == nil {
		return nil, ErrorNoJobProvider
	}
	if s.concurrency == 0 {
		s.concurrency = 1
	}
	if s.netClient == nil {
		s.netClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	}
	s.buffers = sync.Pool{
		New: func() interface{} {
			return new(bytes.Buffer)
		},
	}
	return s, nil
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

// WithHttpClient sets the http client for the scrapemate
func WithHttpClient(client HttpClient) func(*scrapeMate) error {
	return func(s *scrapeMate) error {
		if client == nil {
			return ErrorNoHttpClient
		}
		s.netClient = client
		return nil
	}
}

type scrapeMate struct {
	log         logging.Logger
	ctx         context.Context
	cancelFn    context.CancelCauseFunc
	jobProvider JobProvider
	concurrency int
	netClient   HttpClient
	buffers     sync.Pool
}

func (s *scrapeMate) Start() error {
	s.log.Info("starting scrapemate")
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
		case job := <-jobc:
			s.doJob(ctx, job)
		}
	}
}

func (s *scrapeMate) doJob(ctx context.Context, job IJob) {
	startTime := time.Now().UTC()
	s.log.Debug("starting job", "job", job)
	var (
		resp Response
		ok   bool
	)
	defer func() {
		if r := recover(); r != nil {
			s.log.Error("panic while executing job", "job", job, "error", r)
			panic(r)
			return
		}
		if !ok {
			s.log.Warn("job failed", "job", job, "error", resp.Error, "status", resp.StatusCode)
			return
		}
		s.log.Info("job finished", "job", job, "duration", time.Now().UTC().Sub(startTime))
	}()
	retryPolicy := job.GetRetryPolicy()
	delay := time.Millisecond * 100
	retry := 0
	for {
		resp = s.crawl(ctx, job)
		switch job.DoCheckResponse {
		case nil:
			ok = resp.StatusCode >= 200 && resp.StatusCode < 300
		default:
			ok = job.DoCheckResponse(resp)
		}
		if ok {
			break
		}
		if retry >= job.GetMaxRetries() {
			break
		}
		retry++
		switch retryPolicy {
		case StopScraping:
			s.log.Warn("stopping scraping because of policy")
			s.cancelFn(errors.New("stopping scraping because of policy"))
			return
		case DiscardJob:
			return
		case RetryJob:
			time.Sleep(delay)
			delay = delay * 2
		case RefreshIP:
			// TODO
		}
	}
	if !ok {
		return
	}
	// cache the response if needed
	// process the response
	if err := resp.SetDocument(); err != nil {
		s.log.Error("error while setting document", "error", err)
		return
	}
	job.SetResponse(resp)
	ctx = context.WithValue(ctx, "log", s.log.With("jobid", job.GetID()))
	next, err := job.Process(ctx, nil)
	if err != nil {
		// TODO shall I retry?
		s.log.Error("error while processing job", "error", err)
		return
	}
	for i := range next {
		s.jobProvider.Push(ctx, next[i])
	}
}

func (s *scrapeMate) crawl(ctx context.Context, job IJob) Response {
	jobParams := job.GetUrlParams()
	params := url.Values{}
	for k, v := range jobParams {
		params.Add(k, v)
	}
	u := job.GetURL() + "?" + params.Encode()

	var reqBody *bytes.Buffer
	reqBody = s.getBuffer()
	defer s.putBuffer(reqBody)
	if len(job.GetBody()) > 0 {
		reqBody.Write(job.GetBody())
	}
	var ans Response
	req, err := http.NewRequestWithContext(ctx, job.GetMethod(), u, reqBody)
	if err != nil {
		ans.Error = err
		return ans
	}
	for k, v := range job.GetHeaders() {
		req.Header.Add(k, v)
	}
	resp, err := s.netClient.Do(req)
	if err != nil {
		ans.Error = err
		return ans
	}
	defer func() {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}()
	ans.StatusCode = resp.StatusCode
	ans.Headers = http.Header{}
	for k, v := range resp.Header {
		ans.Headers[k] = v
	}
	var reader io.ReadCloser
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err = gzip.NewReader(resp.Body)
		if err != nil {
			ans.Error = err
			return ans
		}
		defer reader.Close()
	default:
		reader = resp.Body
	}
	ans.Data, ans.Error = ioutil.ReadAll(reader)
	return ans
}

func (s *scrapeMate) getBuffer() *bytes.Buffer {
	b := s.buffers.Get().(*bytes.Buffer)
	b.Reset()
	return b
}

func (s *scrapeMate) putBuffer(buf *bytes.Buffer) {
	s.buffers.Put(buf)
}

func (s *scrapeMate) Done() <-chan struct{} {
	return s.ctx.Done()
}

func (s *scrapeMate) Err() error {
	return context.Cause(s.ctx)
}
