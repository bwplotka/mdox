// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package linktransformer

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bwplotka/mdox/pkg/mdformatter"
	"github.com/efficientgo/core/testutil"
	"github.com/go-kit/log"
	"github.com/gocolly/colly/v2"
	"github.com/prometheus/client_golang/prometheus"
)

func remoteLinksFile(t *testing.T, links ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "links.md")
	var doc []string
	for _, link := range links {
		doc = append(doc, "[link]("+link+")")
	}
	testutil.Ok(t, os.WriteFile(path, []byte(strings.Join(doc, "\n\n")+"\n"), 0600))
	return path
}

func checkRemoteLinks(t *testing.T, ctx context.Context, v mdformatter.LinkTransformer, links ...string) error {
	t.Helper()
	path := remoteLinksFile(t, links...)
	_, err := mdformatter.IsFormatted(ctx, log.NewNopLogger(), []string{path}, mdformatter.WithLinkTransformer(v))
	return err
}

func TestValidator_RetryBehavior(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name         string
		firstStatus  int
		finalStatus  int
		wantRequests int32
		wantError    string
	}{
		{name: "429 recovers", firstStatus: 429, finalStatus: 200, wantRequests: 2},
		{name: "429 does not hide 404", firstStatus: 429, finalStatus: 404, wantRequests: 2, wantError: "status code 404"},
		{name: "429 exhausts retries", firstStatus: 429, finalStatus: 429, wantRequests: 4, wantError: "status code 429 after 3 retries"},
		{name: "500 recovers", firstStatus: 500, finalStatus: 200, wantRequests: 2},
		{name: "502 recovers", firstStatus: 502, finalStatus: 200, wantRequests: 2},
		{name: "503 recovers", firstStatus: 503, finalStatus: 200, wantRequests: 2},
		{name: "504 recovers", firstStatus: 504, finalStatus: 200, wantRequests: 2},
		{name: "503 exhausts retries", firstStatus: 503, finalStatus: 503, wantRequests: 4, wantError: "status code 503 after 3 retries"},
		{name: "EOF recovers", firstStatus: 0, finalStatus: 200, wantRequests: 2},
		{name: "404 is not retried", firstStatus: 404, finalStatus: 200, wantRequests: 1, wantError: "status code 404"},
		{name: "301 follows redirect", firstStatus: 301, finalStatus: 200, wantRequests: 2},
		{name: "307 follows redirect", firstStatus: 307, finalStatus: 200, wantRequests: 2},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var requests int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				status := tc.finalStatus
				if atomic.AddInt32(&requests, 1) == 1 {
					status = tc.firstStatus
				}
				if status == 0 {
					conn, _, err := w.(http.Hijacker).Hijack()
					if err != nil {
						t.Errorf("hijack connection: %v", err)
						return
					}
					if err := conn.Close(); err != nil {
						t.Errorf("close connection: %v", err)
					}
					return
				}
				if status == http.StatusMovedPermanently || status == http.StatusTemporaryRedirect {
					http.Redirect(w, r, "/target", status)
					return
				}
				w.WriteHeader(status)
			}))
			defer srv.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			reg := prometheus.NewRegistry()
			v, err := NewValidator(ctx, log.NewNopLogger(), nil, t.TempDir(), nil, reg)
			testutil.Ok(t, err)
			err = checkRemoteLinks(t, ctx, v, srv.URL)
			if tc.wantError == "" {
				testutil.Ok(t, err)
			} else {
				testutil.NotOk(t, err)
				testutil.Assert(t, strings.Contains(err.Error(), tc.wantError), "unexpected error: %v", err)
			}
			testutil.Equals(t, tc.wantRequests, atomic.LoadInt32(&requests))
			responses := tc.wantRequests
			if tc.firstStatus == 0 {
				responses--
			}
			metrics, err := reg.Gather()
			testutil.Ok(t, err)
			counts := map[string]float64{}
			for _, family := range metrics {
				switch family.GetName() {
				case "mdox_colly_requests_total":
					counts[family.GetName()] = family.GetMetric()[0].GetCounter().GetValue()
				case "mdox_colly_per_domain_latency":
					metric := family.GetMetric()[0]
					testutil.Equals(t, strings.TrimPrefix(srv.URL, "http://"), metric.GetLabel()[0].GetValue())
					counts[family.GetName()] = float64(metric.GetHistogram().GetSampleCount())
				}
			}
			testutil.Equals(t, map[string]float64{
				"mdox_colly_requests_total":     float64(responses),
				"mdox_colly_per_domain_latency": float64(responses),
			}, counts)
		})
	}
}

func TestValidator_RetryAfter(t *testing.T) {
	t.Parallel()
	for _, status := range []int{http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		for _, format := range []string{"seconds", "HTTP date", "malformed"} {
			status, format := status, format
			t.Run(fmt.Sprintf("%d/%s", status, format), func(t *testing.T) {
				t.Parallel()
				var requests int32
				firstResponse := make(chan time.Time, 1)
				retriedAt := make(chan time.Time, 1)
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					now := time.Now()
					if atomic.AddInt32(&requests, 1) > 1 {
						retriedAt <- now
						w.WriteHeader(http.StatusOK)
						return
					}
					minimum := now.Add(2 * time.Second)
					header := "invalid"
					switch format {
					case "seconds":
						header = "3"
						minimum = now.Add(3 * time.Second)
					case "HTTP date":
						minimum = now.Add(4 * time.Second).Truncate(time.Second)
						header = minimum.UTC().Format(http.TimeFormat)
					}
					firstResponse <- minimum
					w.Header().Set("Retry-After", header)
					w.WriteHeader(status)
				}))
				defer srv.Close()

				ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
				defer cancel()
				v, err := NewValidator(ctx, log.NewNopLogger(), nil, t.TempDir(), nil, nil)
				testutil.Ok(t, err)
				testutil.Ok(t, checkRemoteLinks(t, ctx, v, srv.URL))
				testutil.Equals(t, int32(2), atomic.LoadInt32(&requests))
				minimum, actual := <-firstResponse, <-retriedAt
				testutil.Assert(t, !actual.Before(minimum), "retried %v before the minimum delay", minimum.Sub(actual))
			})
		}
	}
}

func TestValidator_RetrySchedulingError(t *testing.T) {
	t.Parallel()
	var requests int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&requests, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	v, err := NewValidator(context.Background(), log.NewNopLogger(), nil, t.TempDir(), nil, nil)
	testutil.Ok(t, err)
	c := v.(*validator).c
	// Reject only the retry, after the initial request has passed Colly's checks.
	c.OnResponseHeaders(func(_ *colly.Response) {
		c.DisallowedURLFilters = []*regexp.Regexp{regexp.MustCompile(".*")}
	})
	err = checkRemoteLinks(t, context.Background(), v, srv.URL)
	testutil.NotOk(t, err)
	testutil.Assert(t, strings.Contains(err.Error(), colly.ErrForbiddenURL.Error()), "unexpected error: %v", err)
	testutil.Equals(t, int32(1), atomic.LoadInt32(&requests))
}

func TestValidator_LongRetryAfter(t *testing.T) {
	t.Parallel()
	for _, header := range []string{"61", "3600", time.Now().Add(24 * time.Hour).UTC().Format(http.TimeFormat), "4294967296"} {
		header := header
		t.Run(header, func(t *testing.T) {
			t.Parallel()
			var requests int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				atomic.AddInt32(&requests, 1)
				w.Header().Set("Retry-After", header)
				w.WriteHeader(http.StatusTooManyRequests)
			}))
			defer srv.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			v, err := NewValidator(ctx, log.NewNopLogger(), nil, t.TempDir(), nil, nil)
			testutil.Ok(t, err)
			err = checkRemoteLinks(t, ctx, v, srv.URL)
			testutil.NotOk(t, err)
			testutil.Assert(t, strings.Contains(err.Error(), "Retry-After") && strings.Contains(err.Error(), "exceeds maximum"), "unexpected error: %v", err)
			testutil.Assert(t, !errors.Is(err, context.DeadlineExceeded), "waited for cancellation instead of reporting the excessive delay")
			testutil.Equals(t, int32(1), atomic.LoadInt32(&requests))
		})
	}
}

func TestValidator_CancelInFlight(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	var requests int32
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		close(started)
		<-r.Context().Done()
	}))
	defer func() {
		cancel()
		srv.Close()
	}()

	v, err := NewValidator(ctx, log.NewNopLogger(), nil, t.TempDir(), nil, nil)
	testutil.Ok(t, err)
	path := remoteLinksFile(t, srv.URL)
	result := make(chan error, 1)
	go func() {
		_, err := mdformatter.IsFormatted(ctx, log.NewNopLogger(), []string{path}, mdformatter.WithLinkTransformer(v))
		result <- err
	}()

	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("request did not start")
	}
	cancel()
	select {
	case err := <-result:
		testutil.NotOk(t, err)
		testutil.Assert(t, errors.Is(err, context.Canceled), "unexpected cancellation error: %v", err)
		testutil.Assert(t, !strings.Contains(err.Error(), "backoff"), "in-flight cancellation reported as backoff: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("canceled request did not finish")
	}
	testutil.Equals(t, int32(1), atomic.LoadInt32(&requests))
}

func TestValidator_BackoffDoesNotBlockOtherLinks(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	releaseHealthy := make(chan struct{})
	var requests int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		if r.URL.Path == "/limited" {
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		select {
		case <-releaseHealthy:
			w.WriteHeader(http.StatusOK)
		case <-r.Context().Done():
		}
	}))
	defer func() {
		cancel()
		srv.Close()
	}()

	v, err := NewValidator(ctx, log.NewNopLogger(), nil, t.TempDir(), nil, nil)
	testutil.Ok(t, err)
	validator := v.(*validator)
	limitedResponse := make(chan *colly.Context, 1)
	validator.c.OnResponseHeaders(func(r *colly.Response) {
		if r.Request.URL.Path == "/limited" {
			limitedResponse <- r.Ctx
		}
	})
	healthyDone := make(chan struct{})
	validator.c.OnScraped(func(r *colly.Response) {
		if r.Request.URL.Path == "/healthy" {
			close(healthyDone)
		}
	})
	result := make(chan error, 1)
	path := remoteLinksFile(t, srv.URL+"/limited", srv.URL+"/healthy")
	go func() {
		_, err := mdformatter.IsFormatted(ctx, log.NewNopLogger(), []string{path}, mdformatter.WithLinkTransformer(v))
		result <- err
	}()

	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	var requestContext *colly.Context
	select {
	case requestContext = <-limitedResponse:
	case <-timer.C:
		t.Fatal("rate-limited response was not received")
	}
	for requestContext.Get(numberOfRetriesKey) != "1" {
		select {
		case <-ticker.C:
		case <-timer.C:
			t.Fatal("rate-limited link did not enter backoff")
		}
	}
	_, err = validator.TransformDestination(mdformatter.SourceContext{Filepath: path, LineNumbers: "duplicate"}, []byte(srv.URL+"/limited"))
	testutil.Ok(t, err)
	close(releaseHealthy)
	select {
	case <-healthyDone:
	case <-timer.C:
		t.Fatal("healthy link was blocked by another link's backoff")
	}
	cancel()
	err = <-result
	testutil.NotOk(t, err)
	testutil.Assert(t, strings.Contains(err.Error(), context.Canceled.Error()), "unexpected cancellation error: %v", err)
	testutil.Assert(t, !strings.Contains(err.Error(), colly.ErrAlreadyVisited.Error()), "duplicate link did not share the final result: %v", err)
	testutil.Equals(t, int32(2), atomic.LoadInt32(&requests))
}

func TestValidator_ConfiguredTimeout(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		config  string
		timeout time.Duration
	}{
		{name: "default", config: "version: 1\n", timeout: 30 * time.Second},
		{name: "short timeout", config: "version: 1\ntimeout: '100ms'\n", timeout: 100 * time.Millisecond},
		{name: "above previous header timeout", config: "version: 1\ntimeout: '35s'\n", timeout: 35 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config, err := ParseConfig([]byte(tc.config))
			testutil.Ok(t, err)
			testutil.Equals(t, tc.timeout, newHTTPTransport(config).ResponseHeaderTimeout)
		})
	}
}
