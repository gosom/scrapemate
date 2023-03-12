package scrapemate

import (
	"context"
	"fmt"
	"time"
)

var _ IJob = (*Job)(nil)

// IJob is a job to be processed by the scrapemate
type IJob interface {
	fmt.Stringer
	// GetID returns the unique identifier of the job.
	GetID() string
	// GetMethod returns the http method to use
	GetMethod() string
	// GetBody returns the body of the request
	GetBody() []byte
	// GetURL returns the url to request
	GetURL() string
	// GetHeaders returns the headers to use
	GetHeaders() map[string]string
	// GetUrlParams returns the url params to use
	GetUrlParams() map[string]string
	// GetTimeout returns the timeout of the job
	GetTimeout() time.Duration
	// GetPriority returns the priority of the job
	GetPriority() int
	// CheckResponse checks the response of the job
	DoCheckResponse(resp Response) bool
	// GetActionOnResponse returns the action to perform on the response
	GetRetryPolicy() RetryPolicy
	// GetMaxRetries returns the max retries of the job
	GetMaxRetries() int
	// SetResponse sets the response of the job
	SetResponse(resp Response)
	// Process processes the job
	Process(ctx context.Context, w JobWriter) ([]IJob, error)
}

type Job struct {
	ID            string
	Method        string
	Body          []byte
	URL           string
	Headers       map[string]string
	UrlParams     map[string]string
	Timeout       time.Duration
	Priority      int
	MaxRetries    int
	CheckResponse func(resp Response) bool
	RetryPolicy   RetryPolicy

	Response Response
}

// String returns the string representation of the job
func (j *Job) String() string {
	return fmt.Sprintf("Job{ID: %s, Method: %s, URL: %s, UrlParams: %v}", j.ID, j.Method, j.URL, j.UrlParams)
}

// Process processes the job
func (j *Job) Process(ctx context.Context, w JobWriter) ([]IJob, error) {
	return nil, nil
}

// SetResponse sets the response of the job
func (j *Job) SetResponse(resp Response) {
	j.Response = resp
}

// CheckResponse checks the response of the job
func (j *Job) DoCheckResponse(resp Response) bool {
	return resp.Error == nil && resp.StatusCode == 200
}

// GetRetryPolicy returns the action to perform on the response
func (j *Job) GetRetryPolicy() RetryPolicy {
	return RetryJob
}

// GetMaxRetry returns the max retry of the job
func (j *Job) GetMaxRetries() int {
	return j.MaxRetries
}

// GetID returns the unique identifier of the job.
func (j *Job) GetID() string {
	return j.ID
}

// GetMethod returns the http method to use
func (j *Job) GetMethod() string {
	return j.Method
}

// GetBody returns the body of the request
func (j *Job) GetBody() []byte {
	return j.Body
}

// GetURL returns the url to request
func (j *Job) GetURL() string {
	return j.URL
}

// GetHeaders returns the headers to use
func (j *Job) GetHeaders() map[string]string {
	return j.Headers
}

// GetUrlParams returns the url params to use
func (j *Job) GetUrlParams() map[string]string {
	return j.UrlParams
}

// GetTimeout returns the timeout of the job
func (j *Job) GetTimeout() time.Duration {
	return j.Timeout
}

// GetPriority returns the priority of the job
func (j *Job) GetPriority() int {
	return j.Priority
}
