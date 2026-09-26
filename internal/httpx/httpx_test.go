package httpx

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func fastClient() *Client { return &Client{Backoff: time.Millisecond} }

func TestGetRetriesThenSucceeds(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch calls.Add(1) {
		case 1:
			w.WriteHeader(http.StatusTooManyRequests)
		case 2:
			w.WriteHeader(http.StatusBadGateway)
		default:
			w.Write([]byte("ok"))
		}
	}))
	defer srv.Close()
	body, err := fastClient().Get(context.Background(), srv.URL)
	if err != nil || string(body) != "ok" || calls.Load() != 3 {
		t.Fatalf("body %q err %v calls %d", body, err, calls.Load())
	}
}

func TestGetGivesUpAsOffline(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	c := fastClient()
	c.Attempts = 3
	_, err := c.Get(context.Background(), srv.URL)
	if !errors.Is(err, ErrOffline) || calls.Load() != 3 {
		t.Fatalf("err %v calls %d", err, calls.Load())
	}
}

func TestGetNetworkErrorIsOffline(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close() // nothing listens there any more
	c := fastClient()
	c.Attempts = 2
	if _, err := c.Get(context.Background(), url); !errors.Is(err, ErrOffline) {
		t.Fatalf("err %v", err)
	}
}

func TestGetFinalStatus(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	_, err := fastClient().Get(context.Background(), srv.URL)
	var se *StatusError
	if !errors.As(err, &se) || se.Code != 401 || calls.Load() != 1 {
		t.Fatalf("err %v calls %d", err, calls.Load())
	}
}

func TestGetHonoursRetryAfter(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Write([]byte("ok"))
	}))
	defer srv.Close()
	start := time.Now()
	if _, err := fastClient().Get(context.Background(), srv.URL); err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d < time.Second {
		t.Fatalf("retried after %v, want ≥ 1 s", d)
	}
}

func TestGetSendsHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(r.Header.Get("Authorization")))
	}))
	defer srv.Close()
	c := fastClient()
	c.Header = http.Header{"Authorization": {"Bearer x"}}
	if body, _ := c.Get(context.Background(), srv.URL); string(body) != "Bearer x" {
		t.Fatalf("body %q", body)
	}
}

func TestLimiter(t *testing.T) {
	l := &Limiter{Rate: 100, Burst: 2}
	start := time.Now()
	for range 6 {
		if err := l.Wait(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	// 2 immediate, then 4 at 10 ms each.
	if d := time.Since(start); d < 35*time.Millisecond {
		t.Fatalf("6 tokens in %v", d)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	slow := &Limiter{Rate: 0.001, Burst: 1}
	slow.Wait(context.Background())
	if err := slow.Wait(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("err %v", err)
	}
}

func TestGetRejectsOversizedBody(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Write(make([]byte, maxBody+1))
	}))
	defer srv.Close()
	if _, err := fastClient().Get(context.Background(), srv.URL); !errors.Is(err, ErrTooLarge) || calls.Load() != 1 {
		t.Fatalf("err %v calls %d", err, calls.Load())
	}
}
