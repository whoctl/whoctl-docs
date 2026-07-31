// Package fetch does the one HTTP thing this repository needs and the standard
// library does not: retry what is worth retrying.
//
// The build fails on an unreadable bundle deliberately — a site quietly missing
// a provider is much harder to notice than a build that stopped. That policy is
// about a provider that is *wrong*, though, and a 503 from a CDN is not. Fresh
// release assets in particular answer 502, 503 and 504 for a minute or so while
// they propagate, which is exactly when a release dispatch arrives, and a build
// that gives up there leaves the registry index a version behind until the
// nightly run.
//
// So: a status that says "this will never work" fails immediately, and one that
// says "not yet" is tried again.
package fetch

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// Attempts is how many times a request is made in total.
const Attempts = 4

// backoff is how long to wait after the nth failure. Roughly ten seconds all
// told, which covers asset propagation without turning a genuinely broken
// registry into a build that hangs. It is a variable so the tests do not.
var backoff = func(attempt int) time.Duration {
	return time.Duration(1<<attempt) * time.Second
}

// Get fetches a URL, retrying a transport error or a server error. The caller
// closes the body.
//
// A 4xx is returned on the first try: a missing bundle, a repository that is
// private, a URL with a typo — none of them get better by asking again, and
// retrying would only delay the message that says so.
func Get(ctx context.Context, client *http.Client, url string, header http.Header) (*http.Response, error) {
	if client == nil {
		client = http.DefaultClient
	}
	var last error

	for attempt := 0; attempt < Attempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt - 1)):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		for k, values := range header {
			for _, v := range values {
				req.Header.Add(k, v)
			}
		}

		resp, err := client.Do(req)
		if err != nil {
			last = err
			continue
		}
		if resp.StatusCode < 500 {
			return resp, nil
		}
		// The body of a 5xx is an error page nobody reads, and leaving it open
		// would leak a connection per attempt.
		resp.Body.Close()
		last = fmt.Errorf("fetching %s: %s", url, resp.Status)
	}
	return nil, fmt.Errorf("fetching %s: giving up after %d attempts: %w", url, Attempts, last)
}
