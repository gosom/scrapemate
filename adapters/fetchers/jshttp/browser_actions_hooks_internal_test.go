package jshttp

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/gosom/scrapemate"
)

func TestRunBrowserActionsClearsRequestAndResponseHooks(t *testing.T) {
	t.Parallel()

	page := &hookPage{}
	ctx := context.Background()

	var (
		requestCalls  int
		responseCalls int
	)

	job1 := &hookJob{
		action: func(_ context.Context, page scrapemate.BrowserPage) scrapemate.Response {
			hooks, ok := page.(scrapemate.RequestHookProvider)
			if !ok {
				t.Fatal("expected hook-capable page")
			}

			hooks.OnRequest(func(_ string, _ map[string]string) {
				requestCalls++
			})
			hooks.OnResponse(func(_ string, _ int, _ map[string]string) {
				responseCalls++
			})

			hookPage, ok := page.(*hookPage)
			if !ok {
				t.Fatal("expected *hookPage")
			}

			hookPage.emitRequest("https://example.test/job-1")
			hookPage.emitResponse("https://example.test/job-1", http.StatusOK)

			return scrapemate.Response{StatusCode: http.StatusOK}
		},
	}

	runBrowserActions(ctx, job1, page)

	if requestCalls != 1 {
		t.Fatalf("expected job 1 request hook to run once, got %d", requestCalls)
	}

	if responseCalls != 1 {
		t.Fatalf("expected job 1 response hook to run once, got %d", responseCalls)
	}

	job2 := &hookJob{
		action: func(_ context.Context, page scrapemate.BrowserPage) scrapemate.Response {
			hookPage, ok := page.(*hookPage)
			if !ok {
				t.Fatal("expected *hookPage")
			}

			hookPage.emitRequest("https://example.test/job-2")
			hookPage.emitResponse("https://example.test/job-2", http.StatusOK)

			return scrapemate.Response{StatusCode: http.StatusOK}
		},
	}

	runBrowserActions(ctx, job2, page)

	if requestCalls != 1 {
		t.Fatalf("expected job 1 request hook to be cleared before job 2, got %d calls", requestCalls)
	}

	if responseCalls != 1 {
		t.Fatalf("expected job 1 response hook to be cleared before job 2, got %d calls", responseCalls)
	}
}

type hookJob struct {
	scrapemate.Job
	action func(context.Context, scrapemate.BrowserPage) scrapemate.Response
}

func (j *hookJob) BrowserActions(ctx context.Context, page scrapemate.BrowserPage) scrapemate.Response {
	return j.action(ctx, page)
}

type hookPage struct {
	requestHooks  []func(string, map[string]string)
	responseHooks []func(string, int, map[string]string)
}

func (p *hookPage) OnRequest(handler func(url string, headers map[string]string)) {
	p.requestHooks = append(p.requestHooks, handler)
}

func (p *hookPage) OnResponse(handler func(url string, statusCode int, headers map[string]string)) {
	p.responseHooks = append(p.responseHooks, handler)
}

func (p *hookPage) ClearNetworkHooks() {
	p.requestHooks = nil
	p.responseHooks = nil
}

func (p *hookPage) emitRequest(url string) {
	for _, hook := range p.requestHooks {
		hook(url, nil)
	}
}

func (p *hookPage) emitResponse(url string, statusCode int) {
	for _, hook := range p.responseHooks {
		hook(url, statusCode, nil)
	}
}

func (p *hookPage) Goto(_ string, _ scrapemate.WaitUntilState) (*scrapemate.PageResponse, error) {
	return &scrapemate.PageResponse{StatusCode: http.StatusOK}, nil
}

func (p *hookPage) URL() string { return "" }

func (p *hookPage) Content() (string, error) { return "", nil }

func (p *hookPage) Reload(_ scrapemate.WaitUntilState) error { return nil }

func (p *hookPage) Screenshot(_ bool) ([]byte, error) { return nil, nil }

func (p *hookPage) Eval(_ string, _ ...any) (any, error) { return nil, nil }

func (p *hookPage) WaitForURL(_ string, _ time.Duration) error { return nil }

func (p *hookPage) WaitForSelector(_ string, _ time.Duration) error { return nil }

func (p *hookPage) WaitForTimeout(_ time.Duration) {}

func (p *hookPage) Locator(_ string) scrapemate.Locator { return nil }

func (p *hookPage) Close() error { return nil }

func (p *hookPage) Unwrap() any { return nil }
