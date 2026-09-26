// Package httpx is the HTTP plumbing shared by the metadata clients: a
// token-bucket rate limiter and GET requests with retries and backoff.
package httpx

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// ErrOffline means the service could not be reached (network down, DNS,
// timeouts, or repeated 429/5xx): the work should be retried later.
var ErrOffline = errors.New("sin conexión")

// StatusError is a final (non-retried) HTTP error status.
type StatusError struct {
	Code int
	URL  string
}

func (e *StatusError) Error() string { return fmt.Sprintf("HTTP %d: %s", e.Code, e.URL) }

// Limiter is a token bucket: Rate tokens per second, holding at most Burst.
type Limiter struct {
	Rate  float64
	Burst float64

	mu     sync.Mutex
	tokens float64
	last   time.Time
	init   bool
}

// Wait blocks until a token is available or ctx ends.
func (l *Limiter) Wait(ctx context.Context) error {
	for {
		l.mu.Lock()
		now := time.Now()
		if !l.init {
			l.tokens, l.last, l.init = l.Burst, now, true
		}
		l.tokens = min(l.Burst, l.tokens+now.Sub(l.last).Seconds()*l.Rate)
		l.last = now
		if l.tokens >= 1 {
			l.tokens--
			l.mu.Unlock()
			return nil
		}
		wait := time.Duration((1 - l.tokens) / l.Rate * float64(time.Second))
		l.mu.Unlock()
		t := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
	}
}

// Client performs rate-limited GET requests with retries.
type Client struct {
	HTTP     *http.Client
	Limiter  *Limiter
	Attempts int           // total tries; 0 means 5
	Backoff  time.Duration // first retry delay, doubled each time; 0 means 1 s
	Header   http.Header   // added to every request
}

// maxBody caps a response so a misbehaving server cannot exhaust memory.
const maxBody = 32 << 20

// Get fetches url and returns the body of a 2xx response. 429, 5xx and
// network errors are retried with exponential backoff (honouring
// Retry-After); when the attempts run out the error wraps ErrOffline. Other
// statuses return a *StatusError right away.
func (c *Client) Get(ctx context.Context, url string) ([]byte, error) {
	attempts, backoff := c.Attempts, c.Backoff
	if attempts == 0 {
		attempts = 5
	}
	if backoff == 0 {
		backoff = time.Second
	}
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	var last error
	for i := range attempts {
		if i > 0 {
			d := backoff << (i - 1)
			var ra *retryAfter
			if errors.As(last, &ra) && ra.d > d {
				d = ra.d
			}
			d += time.Duration(rand.Int64N(int64(d)/4 + 1))
			t := time.NewTimer(d)
			select {
			case <-ctx.Done():
				t.Stop()
				return nil, ctx.Err()
			case <-t.C:
			}
		}
		if c.Limiter != nil {
			if err := c.Limiter.Wait(ctx); err != nil {
				return nil, err
			}
		}
		body, err := c.once(ctx, hc, url)
		if err == nil {
			return body, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		var se *StatusError
		if errors.As(err, &se) {
			return nil, err
		}
		last = err
	}
	return nil, fmt.Errorf("%w: %v", ErrOffline, last)
}

// retryAfter is a retryable status, with the delay the server asked for.
type retryAfter struct {
	code int
	d    time.Duration
}

func (e *retryAfter) Error() string { return "HTTP " + strconv.Itoa(e.code) }

func (c *Client) once(ctx context.Context, hc *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	for k, vs := range c.Header {
		req.Header[k] = vs
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	switch {
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		secs, _ := strconv.Atoi(resp.Header.Get("Retry-After"))
		return nil, &retryAfter{code: resp.StatusCode, d: time.Duration(secs) * time.Second}
	case resp.StatusCode < 200 || resp.StatusCode > 299:
		return nil, &StatusError{Code: resp.StatusCode, URL: url}
	}
	return body, err
}
