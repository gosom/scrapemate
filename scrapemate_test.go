package scrapemate_test

import (
	"context"
	"errors"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/gosom/kit/logging"

	"github.com/gosom/scrapemate"
	"github.com/gosom/scrapemate/mock"
)

type mockedServices struct {
	provider *mock.MockJobProvider
	fetcher  *mock.MockHTTPFetcher
	parser   *mock.MockHTMLParser
	cache    *mock.MockCacher
}

func getMockedServices(t *testing.T) *mockedServices {
	t.Helper()
	mockCtrl := gomock.NewController(t)
	httpFetcher := mock.NewMockHTTPFetcher(mockCtrl)
	provider := mock.NewMockJobProvider(mockCtrl)
	parser := mock.NewMockHTMLParser(mockCtrl)
	cache := mock.NewMockCacher(mockCtrl)

	return &mockedServices{
		provider: provider,
		fetcher:  httpFetcher,
		parser:   parser,
		cache:    cache,
	}
}

func Test_New(t *testing.T) {
	svc := getMockedServices(t)
	t.Run("should return error if no job provider is provided", func(t *testing.T) {
		_, err := scrapemate.New()
		require.Error(t, err)
	})
	t.Run("should return error if no http fetcher is provided", func(t *testing.T) {
		_, err := scrapemate.New(
			scrapemate.WithJobProvider(svc.provider),
		)
		require.Error(t, err)
	})
	t.Run("works with default options", func(t *testing.T) {
		s, err := scrapemate.New(
			scrapemate.WithJobProvider(svc.provider),
			scrapemate.WithHTTPFetcher(svc.fetcher),
		)
		require.NoError(t, err)
		require.NotNil(t, s)
		require.Nil(t, s.Failed())
		require.Equal(t, 1, s.Concurrency())
		require.NotNil(t, s.Results())
	})
}

func Test_New_With_Options(t *testing.T) {
	svc := getMockedServices(t)
	t.Run("with failed", func(t *testing.T) {
		mate, err := scrapemate.New(
			scrapemate.WithJobProvider(svc.provider),
			scrapemate.WithHTTPFetcher(svc.fetcher),
			scrapemate.WithFailed(),
		)
		require.NoError(t, err)
		require.NotNil(t, mate)
		require.NotNil(t, mate.Failed())
	})
	t.Run("with context", func(t *testing.T) {
		ctx, cancel := context.WithCancelCause(context.Background())
		defer cancel(errors.New("test"))
		mate, err := scrapemate.New(
			scrapemate.WithJobProvider(svc.provider),
			scrapemate.WithHTTPFetcher(svc.fetcher),
			scrapemate.WithContext(ctx, cancel),
		)
		require.NoError(t, err)
		require.NotNil(t, mate)
		t.Run("with nil cancel", func(t *testing.T) {
			_, err := scrapemate.New(
				scrapemate.WithJobProvider(svc.provider),
				scrapemate.WithHTTPFetcher(svc.fetcher),
				scrapemate.WithContext(context.Background(), nil),
			)
			require.NoError(t, err)
		})
		t.Run("with nil context", func(t *testing.T) {
			_, err := scrapemate.New(
				scrapemate.WithJobProvider(svc.provider),
				scrapemate.WithHTTPFetcher(svc.fetcher),
				//nolint:staticcheck // ignore error
				scrapemate.WithContext(nil, nil),
			)
			require.Error(t, err)
		})
	})
	t.Run("with concurrency", func(t *testing.T) {
		mate, err := scrapemate.New(
			scrapemate.WithJobProvider(svc.provider),
			scrapemate.WithHTTPFetcher(svc.fetcher),
			scrapemate.WithConcurrency(10),
		)
		require.NoError(t, err)
		require.NotNil(t, mate)
		require.Equal(t, 10, mate.Concurrency())
		t.Run("with concurrency less than 1", func(t *testing.T) {
			_, err := scrapemate.New(
				scrapemate.WithJobProvider(svc.provider),
				scrapemate.WithHTTPFetcher(svc.fetcher),
				scrapemate.WithConcurrency(0),
			)
			require.Error(t, err)
		})
	})
	t.Run("with logger", func(t *testing.T) {
		mate, err := scrapemate.New(
			scrapemate.WithJobProvider(svc.provider),
			scrapemate.WithHTTPFetcher(svc.fetcher),
			scrapemate.WithLogger(logging.Get()),
		)
		require.NoError(t, err)
		require.NotNil(t, mate)
		t.Run("with nil logger", func(t *testing.T) {
			_, err := scrapemate.New(
				scrapemate.WithJobProvider(svc.provider),
				scrapemate.WithHTTPFetcher(svc.fetcher),
				scrapemate.WithLogger(nil),
			)
			require.Error(t, err)
		})
	})
	t.Run("with nil job provider", func(t *testing.T) {
		_, err := scrapemate.New(
			scrapemate.WithHTTPFetcher(svc.fetcher),
			scrapemate.WithJobProvider(nil),
		)
		require.Error(t, err)
	})
	t.Run("with nil http fetcher", func(t *testing.T) {
		_, err := scrapemate.New(
			scrapemate.WithHTTPFetcher(nil),
			scrapemate.WithJobProvider(svc.provider),
		)
		require.Error(t, err)
	})
	t.Run("with html parser", func(t *testing.T) {
		mate, err := scrapemate.New(
			scrapemate.WithJobProvider(svc.provider),
			scrapemate.WithHTTPFetcher(svc.fetcher),
			scrapemate.WithHTMLParser(svc.parser),
		)
		require.NoError(t, err)
		require.NotNil(t, mate)
		t.Run("with nil parser", func(t *testing.T) {
			_, err := scrapemate.New(
				scrapemate.WithJobProvider(svc.provider),
				scrapemate.WithHTTPFetcher(svc.fetcher),
				scrapemate.WithHTMLParser(nil),
			)
			require.Error(t, err)
		})
	})
	t.Run("with cache", func(t *testing.T) {
		mate, err := scrapemate.New(
			scrapemate.WithJobProvider(svc.provider),
			scrapemate.WithHTTPFetcher(svc.fetcher),
			scrapemate.WithCache(svc.cache),
		)
		require.NoError(t, err)
		require.NotNil(t, mate)
		t.Run("with nil cache", func(t *testing.T) {
			_, err := scrapemate.New(
				scrapemate.WithJobProvider(svc.provider),
				scrapemate.WithHTTPFetcher(svc.fetcher),
				scrapemate.WithCache(nil),
			)
			require.Error(t, err)
		})
	})
	t.Run("with exit on inactivity", func(t *testing.T) {
		mate, err := scrapemate.New(
			scrapemate.WithJobProvider(svc.provider),
			scrapemate.WithHTTPFetcher(svc.fetcher),
			scrapemate.WithExitBecauseOfInactivity(1*time.Second),
		)
		require.NoError(t, err)
		require.NotNil(t, mate)
	})
}

func Test_Done_Err(t *testing.T) {
	ctx, cancelFn := context.WithCancelCause(context.Background())
	svc := getMockedServices(t)
	mate, err := scrapemate.New(
		scrapemate.WithJobProvider(svc.provider),
		scrapemate.WithHTTPFetcher(svc.fetcher),
		scrapemate.WithContext(ctx, cancelFn),
	)
	require.NoError(t, err)
	require.NotNil(t, mate)
	cancelFn(errors.New("test"))
	select {
	case <-mate.Done():
	default:
		require.Fail(t, "should be done")
	}

	err = mate.Err()
	require.Error(t, err)
	require.Equal(t, "test", err.Error())
}

func Test_Start(t *testing.T) {
	svc := getMockedServices(t)
	t.Run("exits when inactivity", func(t *testing.T) {
		mate, err := scrapemate.New(
			scrapemate.WithJobProvider(svc.provider),
			scrapemate.WithHTTPFetcher(svc.fetcher),
			scrapemate.WithExitBecauseOfInactivity(time.Millisecond*500),
		)
		require.NoError(t, err)
		require.NotNil(t, mate)

		svc.provider.EXPECT().Jobs(gomock.Any()).DoAndReturn(func(context.Context) (<-chan scrapemate.Job, <-chan error) {
			ch := make(chan scrapemate.Job)
			errch := make(chan error)

			return ch, errch
		})

		mateErr := func() <-chan error {
			errc := make(chan error)
			go func() {
				errc <- mate.Start()
			}()

			return errc
		}

		select {
		case err = <-mateErr():
			require.NoError(t, err)
		case <-time.After(2 * time.Minute):
			require.Fail(t, "should be done")
		}

		require.NoError(t, mate.Err())
	})
	t.Run("exits when context is cancelled", func(t *testing.T) {
		ctx, cancelFn := context.WithCancelCause(context.Background())
		mate, err := scrapemate.New(
			scrapemate.WithJobProvider(svc.provider),
			scrapemate.WithHTTPFetcher(svc.fetcher),
			scrapemate.WithContext(ctx, cancelFn),
		)
		require.NoError(t, err)
		require.NotNil(t, mate)

		svc.provider.EXPECT().Jobs(ctx).DoAndReturn(func(context.Context) (<-chan scrapemate.Job, <-chan error) {
			ch := make(chan scrapemate.Job)
			errch := make(chan error)

			return ch, errch
		})

		errc := func() <-chan error {
			ans := make(chan error, 1)
			go func() {
				defer close(ans)

				ans <- mate.Start()
			}()

			return ans
		}()

		cancelFn(errors.New("test"))

		select {
		case <-mate.Done():
		default:
			require.Fail(t, "should be done")
		}

		err = <-errc

		require.Error(t, err)
		require.Equal(t, "test", err.Error())
		require.Equal(t, "test", mate.Err().Error())
	})
	t.Run("exits when an interrupt signal is received", func(t *testing.T) {
		mate, err := scrapemate.New(
			scrapemate.WithJobProvider(svc.provider),
			scrapemate.WithHTTPFetcher(svc.fetcher),
		)
		require.NoError(t, err)
		require.NotNil(t, mate)
		svc.provider.EXPECT().Jobs(gomock.Any()).DoAndReturn(func(context.Context) (<-chan scrapemate.Job, <-chan error) {
			ch := make(chan scrapemate.Job)
			errch := make(chan error)

			return ch, errch
		})

		mateErr := func() <-chan error {
			errc := make(chan error)
			go func() {
				errc <- mate.Start()
			}()

			return errc
		}

		select {
		case err = <-mateErr():
			require.NoError(t, err)
		case <-time.After(1 * time.Second):
			err = syscall.Kill(syscall.Getpid(), syscall.SIGINT)
			require.NoError(t, err)
		}
		require.NoError(t, mate.Err())
	})
	t.Run("handles job provider errors", func(t *testing.T) {
		ctx, cancelFn := context.WithCancelCause(context.Background())

		mate, err := scrapemate.New(
			scrapemate.WithJobProvider(svc.provider),
			scrapemate.WithHTTPFetcher(svc.fetcher),
			scrapemate.WithContext(ctx, cancelFn),
		)
		require.NoError(t, err)
		require.NotNil(t, mate)

		errch := func() <-chan error {
			ans := make(chan error, 1)

			ans <- errors.New("test")

			return ans
		}()

		ch := func() <-chan scrapemate.IJob {
			ans := make(chan scrapemate.IJob)

			return ans
		}()

		svc.provider.EXPECT().Jobs(gomock.Any()).Return(nil, errch)
		svc.provider.EXPECT().Jobs(gomock.Any()).Return(ch, nil)

		mateErr := func() <-chan error {
			errc := make(chan error)

			go func() {
				errc <- mate.Start()
			}()

			return errc
		}()

		time.Sleep(1100 * time.Millisecond)
		cancelFn(scrapemate.ErrorExitSignal)

		select {
		case err := <-mateErr:
			require.Error(t, err)
			require.Equal(t, scrapemate.ErrorExitSignal, err)
		case <-time.After(1 * time.Second):
			require.Fail(t, "should have exited")
		}
	})
	t.Run("with one job with error", func(t *testing.T) {
		svc := getMockedServices(t)

		ctx, cancel := context.WithCancelCause(context.Background())
		defer cancel(errors.New("defer exit"))

		ch := func() <-chan scrapemate.IJob {
			ans := make(chan scrapemate.IJob, 1)
			j := testJobWithError{
				Job: scrapemate.Job{
					URL: "http://example.com",
				},
			}

			ans <- &j

			return ans
		}()
		errch := func() <-chan error {
			ans := make(chan error, 1)
			return ans
		}()

		svc.provider.EXPECT().Jobs(ctx).Return(ch, errch)
		svc.fetcher.EXPECT().Fetch(gomock.Any(), gomock.Any()).Return(scrapemate.Response{
			StatusCode: 200,
			Body:       []byte("test"),
		})

		mate, err := scrapemate.New(
			scrapemate.WithContext(ctx, cancel),
			scrapemate.WithHTTPFetcher(svc.fetcher),
			scrapemate.WithJobProvider(svc.provider),
			scrapemate.WithFailed(),
		)
		require.NoError(t, err)
		require.NotNil(t, mate)

		mateErr := func() <-chan error {
			errc := make(chan error)

			go func() {
				errc <- mate.Start()
			}()

			return errc
		}()

		failed := mate.Failed()
		select {
		case u := <-failed:
			require.Equal(t, "http://example.com", u.GetURL())
		case <-time.After(2 * time.Second):
			require.Fail(t, "timeout")
		}
		cancel(scrapemate.ErrorExitSignal)
		select {
		case err := <-mateErr:
			require.Equal(t, scrapemate.ErrorExitSignal, err)
		case <-time.After(1 * time.Second):
			require.Fail(t, "timeout")
		}
	})
	t.Run("happy path with next", func(t *testing.T) {
		svc := getMockedServices(t)

		ctx, cancel := context.WithCancelCause(context.Background())
		defer cancel(errors.New("defer exit"))

		jobCh := make(chan scrapemate.IJob, 2)
		j := testJobWithNext{
			Job: scrapemate.Job{
				URL: "http://example.com",
			},
		}

		jobCh <- &j

		errch := func() <-chan error {
			ans := make(chan error, 1)
			return ans
		}()

		svc.provider.EXPECT().Jobs(ctx).Return(jobCh, errch)
		svc.fetcher.EXPECT().Fetch(gomock.Any(), gomock.Any()).Return(scrapemate.Response{
			StatusCode: 200,
			Body:       []byte("test"),
		})
		svc.provider.EXPECT().Push(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, job scrapemate.IJob) error {
			jobCh <- job

			return nil
		})
		svc.fetcher.EXPECT().Fetch(gomock.Any(), gomock.Any()).Return(scrapemate.Response{
			StatusCode: 200,
			Body:       []byte("test"),
		})

		mate, err := scrapemate.New(
			scrapemate.WithContext(ctx, cancel),
			scrapemate.WithHTTPFetcher(svc.fetcher),
			scrapemate.WithJobProvider(svc.provider),
			scrapemate.WithFailed(),
		)
		require.NoError(t, err)
		require.NotNil(t, mate)

		mateErr := func() <-chan error {
			errc := make(chan error)

			go func() {
				errc <- mate.Start()
			}()

			return errc
		}()

		finished := mate.Results()

		doneList, doneErr := func() (l []scrapemate.IJob, err error) {
			for {
				select {
				case j := <-finished:
					l = append(l, j.Job)
					select {
					case j := <-finished:
						l = append(l, j.Job)
						return
					case <-time.After(1 * time.Second):
						err = errors.New("timeout")
						return
					}
				case <-time.After(2 * time.Second):
					err = errors.New("timeout")
					return
				}
			}
		}()

		require.NoError(t, doneErr)
		require.Len(t, doneList, 2)
		require.Equal(t, "http://example.com", doneList[0].GetURL())
		require.Equal(t, "http://example.com/next", doneList[1].GetURL())

		cancel(scrapemate.ErrorExitSignal)

		select {
		case err := <-mateErr:
			require.Equal(t, scrapemate.ErrorExitSignal, err)
		case <-time.After(1 * time.Second):
			require.Fail(t, "timeout")
		}
	})
	t.Run("when push fails", func(t *testing.T) {
		svc := getMockedServices(t)

		ctx, cancel := context.WithCancelCause(context.Background())
		defer cancel(errors.New("defer exit"))

		ch := func() <-chan scrapemate.IJob {
			ans := make(chan scrapemate.IJob, 1)
			j := testJobWithNext{
				Job: scrapemate.Job{
					URL: "http://example.com",
				},
			}
			ans <- &j

			return ans
		}()

		errch := func() <-chan error {
			ans := make(chan error, 1)
			return ans
		}()

		svc.provider.EXPECT().Jobs(ctx).Return(ch, errch)
		svc.fetcher.EXPECT().Fetch(gomock.Any(), gomock.Any()).Return(scrapemate.Response{
			StatusCode: 200,
			Body:       []byte("test"),
		})
		svc.provider.EXPECT().Push(gomock.Any(), gomock.Any()).Return(errors.New("error pushing"))

		mate, err := scrapemate.New(
			scrapemate.WithContext(ctx, cancel),
			scrapemate.WithHTTPFetcher(svc.fetcher),
			scrapemate.WithJobProvider(svc.provider),
			scrapemate.WithFailed(),
		)
		require.NoError(t, err)
		require.NotNil(t, mate)

		mateErr := func() <-chan error {
			errc := make(chan error)
			go func() {
				errc <- mate.Start()
			}()

			return errc
		}()

		failed := mate.Failed()

		select {
		case u := <-failed:
			require.Equal(t, "http://example.com", u.GetURL())
		case <-time.After(1 * time.Second):
			require.Fail(t, "timeout")
		}

		cancel(scrapemate.ErrorExitSignal)

		select {
		case err := <-mateErr:
			require.Equal(t, scrapemate.ErrorExitSignal, err)
		case <-time.After(1 * time.Second):
			require.Fail(t, "timeout")
		}
	})
}

type testJobWithError struct {
	scrapemate.Job
}

func (j *testJobWithError) Process(_ context.Context, _ *scrapemate.Response) (any, []scrapemate.IJob, error) {
	return nil, nil, errors.New("error processing")
}

type testJobWithNext struct {
	scrapemate.Job
}

func (j *testJobWithNext) Process(_ context.Context, _ *scrapemate.Response) (any, []scrapemate.IJob, error) {
	next := &testJob{
		Job: scrapemate.Job{
			URL: "http://example.com/next",
		},
	}

	return nil, []scrapemate.IJob{next}, nil
}

type testJob struct {
	scrapemate.Job
}

func (j *testJob) Process(_ context.Context, _ *scrapemate.Response) (any, []scrapemate.IJob, error) {
	return nil, nil, nil
}

func Test_DoJob(t *testing.T) {
	ctx := context.Background()
	svc := getMockedServices(t)
	job := scrapemate.Job{
		URL: "http://example.com",
	}

	t.Run("when panic", func(t *testing.T) {
		mate, err := scrapemate.New(
			scrapemate.WithHTTPFetcher(svc.fetcher),
			scrapemate.WithJobProvider(svc.provider),
		)
		require.NoError(t, err)
		require.NotNil(t, mate)
		svc.fetcher.EXPECT().Fetch(gomock.Any(), &job).Do(func(context.Context, *scrapemate.Job) {
			panic("test")
		})

		_, _, err = mate.DoJob(ctx, &job)
		require.Error(t, err)
	})
	t.Run("invalidStatusCode+policy:Retry+maxRetries:0", func(t *testing.T) {
		mate, err := scrapemate.New(
			scrapemate.WithHTTPFetcher(svc.fetcher),
			scrapemate.WithJobProvider(svc.provider),
		)
		require.NoError(t, err)
		require.NotNil(t, mate)
		svc.fetcher.EXPECT().Fetch(gomock.Any(), &job).Return(scrapemate.Response{
			StatusCode: 400,
			Body:       []byte("test"),
		})

		_, _, err = mate.DoJob(ctx, &job)
		require.Error(t, err)
	})
	t.Run("invalidStatusCode+policy:Retry+maxRetries:1", func(t *testing.T) {
		mate, err := scrapemate.New(
			scrapemate.WithHTTPFetcher(svc.fetcher),
			scrapemate.WithJobProvider(svc.provider),
		)
		require.NoError(t, err)
		require.NotNil(t, mate)

		job2 := job

		job2.MaxRetries = 1

		svc.fetcher.EXPECT().Fetch(gomock.Any(), &job2).Return(scrapemate.Response{
			StatusCode: 400,
			Body:       []byte("test"),
		}).Times(2)

		_, _, err = mate.DoJob(ctx, &job2)
		require.Error(t, err)
	})
	t.Run("invalidStatusCode+policy:Retry+maxRetries:10-testMax5", func(t *testing.T) {
		mate, err := scrapemate.New(
			scrapemate.WithHTTPFetcher(svc.fetcher),
			scrapemate.WithJobProvider(svc.provider),
		)
		require.NoError(t, err)
		require.NotNil(t, mate)

		job2 := job

		job2.MaxRetries = 10
		job2.MaxRetryDelay = 600 * time.Millisecond

		svc.fetcher.EXPECT().Fetch(gomock.Any(), &job2).Return(scrapemate.Response{
			StatusCode: 400,
			Body:       []byte("test"),
		}).Times(6)

		_, _, err = mate.DoJob(ctx, &job2)
		require.Error(t, err)
	})
	t.Run("customDoCheckResponse", func(t *testing.T) {
		mate, err := scrapemate.New(
			scrapemate.WithHTTPFetcher(svc.fetcher),
			scrapemate.WithJobProvider(svc.provider),
		)
		require.NoError(t, err)
		require.NotNil(t, mate)

		job2 := scrapemate.Job{
			URL: "http://example.com",
			CheckResponse: func(response *scrapemate.Response) bool {
				return response.StatusCode == 301
			},
		}

		svc.fetcher.EXPECT().Fetch(gomock.Any(), &job2).Return(scrapemate.Response{
			StatusCode: 301,
			Body:       []byte("test"),
		})

		_, _, err = mate.DoJob(ctx, &job2)
		require.NoError(t, err)
	})
	t.Run("invalidStatusCode+policy:StopScraping+maxRetries:0", func(t *testing.T) {
		mate, err := scrapemate.New(
			scrapemate.WithHTTPFetcher(svc.fetcher),
			scrapemate.WithJobProvider(svc.provider),
		)
		require.NoError(t, err)
		require.NotNil(t, mate)

		job2 := scrapemate.Job{
			URL:         "http://example.com",
			RetryPolicy: scrapemate.StopScraping,
		}
		svc.fetcher.EXPECT().Fetch(gomock.Any(), &job2).Return(scrapemate.Response{
			StatusCode: 400,
			Body:       []byte("test"),
		})

		_, _, err = mate.DoJob(ctx, &job2)
		require.Error(t, err)

		var ctxDone bool

		select {
		case <-mate.Done():
			ctxDone = true
		default:
		}

		require.True(t, ctxDone)
		require.Error(t, mate.Err())
	})
	t.Run("invalidStatusCode+policy:DiscardJob+maxRetries:0", func(t *testing.T) {
		mate, err := scrapemate.New(
			scrapemate.WithHTTPFetcher(svc.fetcher),
			scrapemate.WithJobProvider(svc.provider),
		)
		require.NoError(t, err)
		require.NotNil(t, mate)

		job2 := scrapemate.Job{
			URL:         "http://example.com",
			RetryPolicy: scrapemate.DiscardJob,
		}
		svc.fetcher.EXPECT().Fetch(gomock.Any(), &job2).Return(scrapemate.Response{
			StatusCode: 400,
			Body:       []byte("test"),
		})

		_, _, err = mate.DoJob(ctx, &job2)
		require.Error(t, err)

		var ctxDone bool

		select {
		case <-mate.Done():
			ctxDone = true
		default:
		}

		require.False(t, ctxDone)
	})
	t.Run("successResponse+parseError", func(t *testing.T) {
		mate, err := scrapemate.New(
			scrapemate.WithHTTPFetcher(svc.fetcher),
			scrapemate.WithJobProvider(svc.provider),
			scrapemate.WithHTMLParser(svc.parser),
		)

		require.NoError(t, err)
		require.NotNil(t, mate)
		svc.fetcher.EXPECT().Fetch(gomock.Any(), &job).Return(scrapemate.Response{
			StatusCode: 200,
			Body:       []byte("<html"),
		})
		svc.parser.EXPECT().Parse(gomock.Any(), gomock.Any()).Return(nil, errors.New("test"))

		_, _, err = mate.DoJob(ctx, &job)
		require.Error(t, err)
	})
	t.Run("success+cache+parseError", func(t *testing.T) {
		mate, err := scrapemate.New(
			scrapemate.WithHTTPFetcher(svc.fetcher),
			scrapemate.WithJobProvider(svc.provider),
			scrapemate.WithHTMLParser(svc.parser),
			scrapemate.WithCache(svc.cache),
		)

		require.NoError(t, err)
		require.NotNil(t, mate)

		svc.cache.EXPECT().Get(gomock.Any(), job.GetCacheKey()).Return(scrapemate.Response{
			StatusCode: 200,
			Body:       []byte("<html></html>"),
		}, nil)
		svc.parser.EXPECT().Parse(gomock.Any(), gomock.Any()).Return(nil, errors.New("test"))

		_, _, err = mate.DoJob(ctx, &job)
		require.Error(t, err)
	})
	t.Run("success+cacheError+parseError", func(t *testing.T) {
		mate, err := scrapemate.New(
			scrapemate.WithHTTPFetcher(svc.fetcher),
			scrapemate.WithJobProvider(svc.provider),
			scrapemate.WithHTMLParser(svc.parser),
			scrapemate.WithCache(svc.cache),
		)

		require.NoError(t, err)
		require.NotNil(t, mate)

		svc.cache.EXPECT().Get(gomock.Any(), job.GetCacheKey()).Return(scrapemate.Response{
			StatusCode: 200,
			Body:       []byte("<html></html>"),
		}, errors.New("cache error"))
		svc.fetcher.EXPECT().Fetch(gomock.Any(), &job).Return(scrapemate.Response{
			StatusCode: 200,
			Body:       []byte("<html"),
		})
		svc.cache.EXPECT().Set(gomock.Any(), job.GetCacheKey(), gomock.Any()).Return(nil)
		svc.parser.EXPECT().Parse(gomock.Any(), gomock.Any()).Return(nil, errors.New("test"))

		_, _, err = mate.DoJob(ctx, &job)
		require.Error(t, err)
	})
}
