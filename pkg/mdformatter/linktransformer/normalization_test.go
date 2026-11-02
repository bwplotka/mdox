// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package linktransformer

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bwplotka/mdox/pkg/cache"
	"github.com/efficientgo/core/testutil"
	"github.com/go-kit/log"
	"github.com/gocolly/colly/v2"
)

func TestValidator_NormalizedURLs(t *testing.T) {
	t.Parallel()
	for _, status := range []int{http.StatusOK, http.StatusNotFound} {
		for _, variant := range []string{"empty path", "uppercase host", "dot segments", "empty fragment", "encoded user info"} {
			t.Run(fmt.Sprintf("%d/%s", status, variant), func(t *testing.T) {
				t.Parallel()
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(status)
				}))
				defer srv.Close()
				dest := srv.URL
				switch variant {
				case "uppercase host":
					dest = strings.Replace(srv.URL, "127.0.0.1", "LOCALHOST", 1) + "/link"
				case "dot segments":
					dest += "/docs/../link"
				case "empty fragment":
					dest += "/#"
				case "encoded user info":
					dest = strings.Replace(srv.URL, "http://", "http://%75ser:%70ass@", 1) + "/link"
				}
				v, err := NewValidator(context.Background(), log.NewNopLogger(), nil, t.TempDir(), nil, nil)
				testutil.Ok(t, err)
				err = checkRemoteLinks(t, context.Background(), v, dest)
				if status == http.StatusOK {
					testutil.Ok(t, err)
					return
				}
				testutil.NotOk(t, err)
				testutil.Assert(t, strings.Contains(err.Error(), "status code 404"), "unexpected error: %v", err)
			})
		}
	}
}

func TestValidator_NormalizedURLsShareResult(t *testing.T) {
	t.Parallel()
	for _, status := range []int{http.StatusOK, http.StatusNotFound} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			t.Parallel()
			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requests.Add(1)
				w.WriteHeader(status)
			}))
			defer srv.Close()
			v, err := NewValidator(context.Background(), log.NewNopLogger(), nil, t.TempDir(), nil, nil)
			testutil.Ok(t, err)
			err = checkRemoteLinks(t, context.Background(), v, srv.URL, srv.URL+"/", srv.URL+"/docs/..")
			if status == http.StatusOK {
				testutil.Ok(t, err)
			} else {
				testutil.NotOk(t, err)
				testutil.Equals(t, 3, strings.Count(err.Error(), "status code 404"), "all destinations should share the failed result")
			}
			testutil.Equals(t, int32(1), requests.Load(), "normalized destinations should share one request")
		})
	}
}

func TestValidator_NormalizedURLsUseCache(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	dir := t.TempDir()
	storage := &cache.SQLite3Storage{Filename: filepath.Join(dir, "links.db")}
	v, err := NewValidator(context.Background(), log.NewNopLogger(), []byte("version: 1\ncache:\n  type: sqlite\n"), dir, storage, nil)
	testutil.Ok(t, err)
	t.Cleanup(func() { testutil.Ok(t, storage.Close()) })
	testutil.Ok(t, storage.CacheURL(srv.URL+"/"))
	testutil.Ok(t, checkRemoteLinks(t, context.Background(), v, srv.URL, srv.URL+"/"))
	testutil.Equals(t, int32(0), requests.Load(), "normalized destinations should use the cached result")
}

func TestValidator_AbortedRequest(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	v, err := NewValidator(context.Background(), log.NewNopLogger(), nil, t.TempDir(), nil, nil)
	testutil.Ok(t, err)
	v.(*validator).c.OnRequest(func(r *colly.Request) { r.Abort() })
	err = checkRemoteLinks(t, context.Background(), v, srv.URL)
	testutil.NotOk(t, err)
	testutil.Assert(t, strings.Contains(err.Error(), "did not complete validation"), "unexpected error: %v", err)
	testutil.Equals(t, int32(0), requests.Load())
}

func TestValidator_RedirectedResult(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		status int
		retry  bool
	}{
		{name: "success", status: http.StatusOK},
		{name: "failure", status: http.StatusNotFound},
		{name: "retry recovers", status: http.StatusOK, retry: true},
		{name: "retry fails", status: http.StatusNotFound, retry: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var attempts atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/origin" {
					http.Redirect(w, r, "/target", http.StatusMovedPermanently)
					return
				}
				if attempts.Add(1) == 1 && tc.retry {
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				}
				w.WriteHeader(tc.status)
			}))
			defer srv.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			v, err := NewValidator(ctx, log.NewNopLogger(), nil, t.TempDir(), nil, nil)
			testutil.Ok(t, err)
			dest := srv.URL + "/origin"
			err = checkRemoteLinks(t, ctx, v, dest)
			if tc.status == http.StatusOK {
				testutil.Ok(t, err)
			} else {
				testutil.NotOk(t, err)
				testutil.Assert(t, strings.Contains(err.Error(), fmt.Sprintf("%q not accessible; status code 404", dest)), "unexpected error: %v", err)
			}
			wantAttempts := int32(1)
			if tc.retry {
				wantAttempts++
			}
			testutil.Equals(t, wantAttempts, attempts.Load())
		})
	}
}
