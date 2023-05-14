package quotes

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/PuerkitoBio/goquery"
	"github.com/gosom/scrapemate"
)

type LoginJob struct {
	scrapemate.Job
}

func NewLoginJob(username, password, token string) *LoginJob {
	data := url.Values{
		"csrf_token": {token},
		"username":   {username},
		"password":   {password},
	}
	body := []byte(data.Encode())
	return &LoginJob{
		Job: scrapemate.Job{
			URL:    "https://quotes.toscrape.com/login",
			Method: http.MethodPost,
			Headers: map[string]string{
				"Content-Type": "application/x-www-form-urlencoded",
			},
			Body:       body,
			MaxRetries: 1,
		},
	}
}

func (o *LoginJob) Process(_ context.Context, resp *scrapemate.Response) (any, []scrapemate.IJob, error) {

	doc, ok := resp.Document.(*goquery.Document)
	if !ok {
		return nil, nil, fmt.Errorf("invalid document type %T expected *goquery.Document", resp.Document)
	}

	if err := CheckLogin(doc); err != nil {
		return nil, nil, err
	}

	return nil, nil, nil
}

type LoginCRSFToken struct {
	scrapemate.Job
}

func NewLoginCRSFToken() *LoginCRSFToken {
	return &LoginCRSFToken{
		Job: scrapemate.Job{
			URL:        "https://quotes.toscrape.com/login",
			Method:     http.MethodGet,
			MaxRetries: 1,
		},
	}
}

// Process will extract the CSRF token from the login page and will create a new login job with the token
func (o *LoginCRSFToken) Process(ctx context.Context, resp *scrapemate.Response) (any, []scrapemate.IJob, error) {
	log := scrapemate.GetLoggerFromContext(ctx)
	log.Info("processing LoginCRSFToken job")

	doc, ok := resp.Document.(*goquery.Document)
	if !ok {
		return nil, nil, fmt.Errorf("invalid document type %T expected *goquery.Document", resp.Document)
	}

	token, ok := doc.Find("input[name='csrf_token']").First().Attr("value")
	if !ok {
		return nil, nil, errors.New("could not find csrf token")
	}

	next := []scrapemate.IJob{
		NewLoginJob("admin", "admin", token),
	}

	return nil, next, nil
}

func CheckLogin(doc *goquery.Document) error {
	sel := `div.header-box p>a`
	el := doc.Find(sel)
	if el.Length() == 0 {
		return errors.New("no login element found")
	}

	txt := el.Text()
	if txt != "Logout" {
		return fmt.Errorf("invalid text %s", txt)
	}

	return nil
}
