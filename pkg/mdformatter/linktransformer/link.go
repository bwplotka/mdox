// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package linktransformer

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwplotka/mdox/pkg/cache"
	"github.com/bwplotka/mdox/pkg/mdformatter"
	"github.com/efficientgo/core/merrors"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/gocolly/colly/v2"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var remoteLinkPrefixRe = regexp.MustCompile(`^http[s]?://`)

type LookupError error

var (
	FileNotFoundErr = LookupError(errors.New("file not found"))
	IDNotFoundErr   = LookupError(errors.New("file exists, but does not have such id"))
)

type linktransformerMetrics struct {
	localLinksChecked     prometheus.Counter
	remoteLinksChecked    prometheus.Counter
	roundTripVisitedLinks prometheus.Counter
	roundTripCachedLinks  prometheus.Counter
	githubSkippedLinks    prometheus.Counter
	ignoreSkippedLinks    prometheus.Counter

	collyRequests         *prometheus.CounterVec
	collyPerDomainLatency *prometheus.HistogramVec
}

func newLinktransformerMetrics(reg *prometheus.Registry) *linktransformerMetrics {
	l := &linktransformerMetrics{}

	l.localLinksChecked = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "mdox_local_links_total",
		Help: "The total number of local links which were checked",
	})
	l.remoteLinksChecked = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "mdox_remote_links_total",
		Help: "The total number of remote links which were checked",
	})
	l.roundTripVisitedLinks = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "mdox_round_trip_visited_links_total",
		Help: "The total number of links which were roundtrip checked",
	})
	l.roundTripCachedLinks = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "mdox_round_trip_cached_links_total",
		Help: "The total number of links which cached in SQLite db",
	})
	l.githubSkippedLinks = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "mdox_github_skipped_links_total",
		Help: "The total number of links which were github checked",
	})
	l.ignoreSkippedLinks = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "mdox_ignore_skipped_links_total",
		Help: "The total number of links which were ignore checked",
	})

	l.collyRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "mdox_colly_requests_total"},
		[]string{},
	)
	l.collyPerDomainLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "mdox_colly_per_domain_latency", Buckets: prometheus.DefBuckets},
		[]string{"domain"},
	)

	if reg != nil {
		reg.MustRegister(l.localLinksChecked, l.remoteLinksChecked, l.roundTripVisitedLinks, l.roundTripCachedLinks, l.githubSkippedLinks, l.ignoreSkippedLinks, l.collyRequests, l.collyPerDomainLatency)
	}
	return l
}

const (
	originalURLKey     = "originalURLKey"
	numberOfRetriesKey = "retryKey"
)

type chain struct {
	chain []mdformatter.LinkTransformer
}

func NewChain(c ...mdformatter.LinkTransformer) mdformatter.LinkTransformer {
	return &chain{chain: c}
}

func (l *chain) TransformDestination(ctx mdformatter.SourceContext, destination []byte) (_ []byte, err error) {
	for _, c := range l.chain {
		destination, err = c.TransformDestination(ctx, destination)
		if err != nil {
			return nil, err
		}
	}
	return destination, nil
}

func (l *chain) Close(ctx mdformatter.SourceContext) error {
	errs := merrors.New()
	for _, c := range l.chain {
		errs.Add(c.Close(ctx))
	}
	return errs.Err()
}

type localizer struct {
	address   *regexp.Regexp
	anchorDir string

	localLinksByFile localLinksCache

	logger log.Logger
}

// NewLocalizer returns mdformatter.LinkTransformer that transforms links that matches address via given regexp to local markdown file path (if exists).
func NewLocalizer(logger log.Logger, address *regexp.Regexp, anchorDir string) mdformatter.LinkTransformer {
	return &localizer{logger: logger, address: address, anchorDir: anchorDir, localLinksByFile: map[string]*[]string{}}
}

func (l *localizer) TransformDestination(ctx mdformatter.SourceContext, destination []byte) (_ []byte, err error) {
	matches := remoteLinkPrefixRe.FindAllIndex(destination, 1)
	if matches != nil {
		// URLs. Remove http/https prefix.
		newDest := string(destination[matches[0][1]:])
		// NOTE: We don't check if passed regexp does not make sense (it's empty string etc).
		matches = l.address.FindAllStringIndex(newDest, 1)
		if matches == nil {
			return destination, nil
		}

		// Remove matched address.
		newDest = filepath.Join(l.anchorDir, newDest[matches[0][1]:])
		if err := l.localLinksByFile.Lookup(newDest); err != nil {
			level.Debug(l.logger).Log("msg", "attempted localization failed, no such local link; skipping", "err", err)
			return destination, nil
		}
		// NOTE: This assumes GetAnchorDir was used, so we validated if docPath is in the path of anchorDir.
		return absLinkToRelLink(newDest, ctx.Filepath)
	}

	// Relative or absolute path.
	newDest := absLocalLink(l.anchorDir, ctx.Filepath, string(destination))

	if err := l.localLinksByFile.Lookup(newDest); err != nil {
		level.Debug(l.logger).Log("msg", "attempted localization failed, no such local link; skipping", "err", err)
		return destination, nil
	}
	// NOTE: This assumes GetAnchorDir was used, so we validated if docPath is in the path of anchorDir.
	return absLinkToRelLink(newDest, ctx.Filepath)
}

func (l *localizer) Close(mdformatter.SourceContext) error { return nil }

type validator struct {
	logger         log.Logger
	anchorDir      string
	validateConfig Config

	localLinks  localLinksCache
	rMu         sync.RWMutex
	remoteLinks map[string]error
	c           *colly.Collector
	storage     *cache.SQLite3Storage

	futureMu    sync.Mutex
	destFutures map[futureKey]*futureResult

	l           *linktransformerMetrics
	transportFn func(url string) http.RoundTripper
}

type futureKey struct {
	filepath, dest, lineNumbers string
}

type futureResult struct {
	// function giving result, promised after colly.Wait.
	resultFn func() error
	cases    int
}

// NewValidator returns mdformatter.LinkTransformer that crawls all links.
// TODO(bwplotka): Add optimization and debug modes - this is the main source of latency and pain.
func NewValidator(ctx context.Context, logger log.Logger, linksValidateConfig []byte, anchorDir string, storage *cache.SQLite3Storage, reg *prometheus.Registry) (mdformatter.LinkTransformer, error) {
	var err error
	config := Config{}
	if string(linksValidateConfig) != "" {
		config, err = ParseConfig(linksValidateConfig)
		if err != nil {
			return nil, err
		}
	}

	linktransformerMetrics := newLinktransformerMetrics(reg)
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		ExpectContinueTimeout: 5 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}
	if config.HostMaxConns != nil {
		transport.MaxConnsPerHost = *config.HostMaxConns
	}
	v := &validator{
		logger:         logger,
		anchorDir:      anchorDir,
		validateConfig: config,
		localLinks:     map[string]*[]string{},
		remoteLinks:    map[string]error{},
		c:              colly.NewCollector(colly.Async(), colly.StdlibContext(ctx)),
		storage:        nil,
		destFutures:    map[futureKey]*futureResult{},
		l:              linktransformerMetrics,
		transportFn: func(u string) http.RoundTripper {
			parsed, err := url.Parse(u)
			if err != nil {
				panic(err)
			}
			return promhttp.InstrumentRoundTripperCounter(
				linktransformerMetrics.collyRequests,
				promhttp.InstrumentRoundTripperDuration(
					linktransformerMetrics.collyPerDomainLatency.MustCurryWith(prometheus.Labels{"domain": parsed.Host}),
					transport,
				),
			)
		},
	}

	// Set very soft limits.
	// E.g github has 50-5000 https://docs.github.com/en/free-pro-team@latest/rest/reference/rate-limit limit depending
	// on api (only search is below 100).
	if config.Timeout != "" {
		v.c.SetRequestTimeout(config.timeout)
	}

	if v.validateConfig.Cache.Type != none && storage != nil {
		v.storage = storage
		if err = v.storage.Init(v.validateConfig.Cache.validity, v.validateConfig.Cache.jitter); err != nil {
			return nil, err
		}
	}

	limitRule := &colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 100,
	}
	if config.Parallelism > 0 {
		limitRule.Parallelism = config.Parallelism
	}
	if config.RandomDelay != "" {
		limitRule.RandomDelay = config.randomDelay
	}
	if err := v.c.Limit(limitRule); err != nil {
		return nil, err
	}
	v.c.OnRequest(func(request *colly.Request) {
		v.rMu.Lock()
		defer v.rMu.Unlock()
		request.Ctx.Put(originalURLKey, request.URL.String())
	})
	v.c.OnScraped(func(response *colly.Response) {
		v.rMu.Lock()
		defer v.rMu.Unlock()
		if v.storage != nil {
			if err := v.storage.CacheURL(response.Ctx.Get(originalURLKey)); err != nil {
				v.remoteLinks[response.Ctx.Get(originalURLKey)] = errors.Wrapf(err, "remote link not saved to cache %v", response.Ctx.Get(originalURLKey))
			}
		}
		v.remoteLinks[response.Ctx.Get(originalURLKey)] = nil
	})
	v.c.OnError(func(response *colly.Response, err error) {
		v.rMu.Lock()
		defer v.rMu.Unlock()
		retriesStr := response.Ctx.Get(numberOfRetriesKey)
		retries, _ := strconv.Atoi(retriesStr)
		switch response.StatusCode {
		case http.StatusTooManyRequests:
			if retries > 0 {
				break
			}
			var retryAfterSeconds int
			// Retry calls same methods as Visit and makes request with same options.
			// So retryKey is incremented here if onError is called again after Retry. By default retries once.
			response.Ctx.Put(numberOfRetriesKey, strconv.Itoa(retries+1))
			retryAfterSeconds, convErr := strconv.Atoi(response.Headers.Get("Retry-After"))
			if convErr != nil {
				retryAfterSeconds = 1
			}
			select {
			case <-time.After(time.Duration(retryAfterSeconds) * time.Second):
			case <-v.c.Context.Done():
				return
			}

			if retryErr := response.Request.Retry(); retryErr != nil {
				v.remoteLinks[response.Ctx.Get(originalURLKey)] = errors.Wrapf(err, "remote link retry %v", response.Ctx.Get(originalURLKey))
				break
			}
			v.remoteLinks[response.Ctx.Get(originalURLKey)] = errors.Wrapf(err, "%q rate limited even after retry; status code %v", response.Request.URL.String(), response.StatusCode)
		// 0 StatusCode means error on call side.
		case http.StatusMovedPermanently, http.StatusTemporaryRedirect, http.StatusServiceUnavailable, 0:
			if retries > 0 {
				break
			}
			response.Ctx.Put(numberOfRetriesKey, strconv.Itoa(retries+1))

			if retryErr := response.Request.Retry(); retryErr != nil {
				v.remoteLinks[response.Ctx.Get(originalURLKey)] = errors.Wrapf(err, "remote link retry %v", response.Ctx.Get(originalURLKey))
				break
			}
			v.remoteLinks[response.Ctx.Get(originalURLKey)] = errors.Wrapf(err, "%q not accessible even after retry; status code %v", response.Request.URL.String(), response.StatusCode)
		default:
			v.remoteLinks[response.Ctx.Get(originalURLKey)] = errors.Wrapf(err, "%q not accessible; status code %v", response.Request.URL.String(), response.StatusCode)
		}
	})
	return v, nil
}

// MustNewValidator returns mdformatter.LinkTransformer that crawls all links.
func MustNewValidator(logger log.Logger, linksValidateConfig []byte, anchorDir string, storage *cache.SQLite3Storage) mdformatter.LinkTransformer {
	v, err := NewValidator(context.TODO(), logger, linksValidateConfig, anchorDir, storage, nil)
	if err != nil {
		panic(err)
	}
	return v
}

func (v *validator) TransformDestination(ctx mdformatter.SourceContext, destination []byte) (_ []byte, err error) {
	v.visit(ctx.Filepath, string(destination), ctx.LineNumbers)
	return destination, nil
}

func (v *validator) Close(ctx mdformatter.SourceContext) error {
	v.c.Wait()

	var keys []futureKey
	for k := range v.destFutures {
		if k.filepath != ctx.Filepath {
			continue
		}
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].filepath+keys[i].dest > keys[j].filepath+keys[j].dest
	})

	merr := merrors.New()
	base, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, "resolve working dir")
	}
	path, err := filepath.Rel(base, ctx.Filepath)
	if err != nil {
		return errors.Wrap(err, "find relative path")
	}

	for _, k := range keys {
		f := v.destFutures[k]
		if err := f.resultFn(); err != nil {
			if f.cases == 1 {
				merr.Add(errors.Wrapf(err, "%v:%v", path, k.lineNumbers))
				continue
			}
			merr.Add(errors.Wrapf(err, "%v:%v (%v occurrences)", path, k.lineNumbers, f.cases))
		}
	}
	return merr.Err()
}

func (v *validator) checkLocal(k futureKey) bool {
	v.l.localLinksChecked.Inc()
	// Check if link is email address.
	if email := strings.TrimPrefix(k.dest, "mailto:"); email != k.dest {
		if isValidEmail(email) {
			return true
		}
		v.destFutures[k].resultFn = func() error { return errors.Errorf("provided mailto link is not a valid email, got %v", k.dest) }
		return false
	}

	// Relative or absolute path. Check if exists.
	newDest := absLocalLink(v.anchorDir, k.filepath, k.dest)

	// Local link. Check if exists.
	if err := v.localLinks.Lookup(newDest); err != nil {
		v.destFutures[k].resultFn = func() error { return errors.Wrapf(err, "link %v, normalized to", k.dest) }
		return false
	}
	return true
}

func (v *validator) visit(filepath string, dest string, lineNumbers string) {
	v.futureMu.Lock()
	defer v.futureMu.Unlock()
	k := futureKey{filepath: filepath, dest: dest, lineNumbers: lineNumbers}
	if _, ok := v.destFutures[k]; ok {
		v.destFutures[k].cases++
		return
	}
	v.destFutures[k] = &futureResult{cases: 1, resultFn: func() error { return nil }}
	if !v.validateConfig.ExplicitLocalValidators {
		matches := remoteLinkPrefixRe.FindAllStringIndex(dest, 1)
		if matches == nil {
			v.checkLocal(k)
			return
		}
		v.l.remoteLinksChecked.Inc()
	}

	validator := v.validateConfig.GetValidatorForURL(dest)
	if validator != nil {
		matched, err := validator.IsValid(k, v)
		if matched && err == nil {
			return
		}
	}
}

// isValidEmail checks email structure and domain.
func isValidEmail(email string) bool {
	// Check length.
	if len(email) < 3 && len(email) > 254 {
		return false
	}
	// Regex from https://www.w3.org/TR/2016/REC-html51-20161101/sec-forms.html#email-state-typeemail.
	var emailRe = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+\\/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")
	if !emailRe.MatchString(email) {
		return false
	}
	// Check email domain.
	domain := strings.Split(email, "@")
	mx, err := net.LookupMX(domain[1])
	if err != nil || len(mx) == 0 {
		return false
	}
	return true
}

type localLinksCache map[string]*[]string

// Lookup looks for given link in local anchorDir. It returns error if link can't be found.
func (l localLinksCache) Lookup(absLink string) error {
	splitWith := "#"
	if strings.Contains(absLink, "/#") {
		splitWith = "/#"
	}

	absLinkSplit := strings.Split(absLink, splitWith)
	ids, ok := l[absLinkSplit[0]]
	if !ok {
		if err := l.addRelLinks(absLinkSplit[0]); err != nil {
			return err
		}
		ids = l[absLinkSplit[0]]
	}
	if ids == nil {
		return errors.Wrapf(FileNotFoundErr, "%v", absLinkSplit[0])
	}

	if len(absLinkSplit) == 1 {
		return nil
	}

	for _, id := range *ids {
		if strings.Compare(id, absLinkSplit[1]) == 0 {
			return nil
		}
	}
	return errors.Wrapf(IDNotFoundErr, "link %v, existing ids: %v", absLink, *ids)
}

func (l localLinksCache) addRelLinks(localLink string) error {
	// Add item for negative caching.
	l[localLink] = nil

	st, err := os.Stat(localLink)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return errors.Wrapf(err, "failed to stat %v", localLink)
	}

	if st.IsDir() {
		// Dir present, cache presence.
		ids := make([]string, 0)
		l[localLink] = &ids
		return nil
	}

	file, err := os.Open(localLink)
	if err != nil {
		return errors.Wrapf(err, "failed to open file %v", localLink)
	}
	defer file.Close()

	// File present, cache presence.
	ids := make([]string, 0)

	var b []byte
	reader := bufio.NewReader(file)
	for {
		b, err = reader.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				return errors.Wrapf(err, "failed to read file %v", localLink)
			}
			break
		}

		if bytes.HasPrefix(b, []byte(`#`)) {
			ids = append(ids, toHeaderID(b))
		}
	}

	l[localLink] = &ids
	return nil
}

func toHeaderID(header []byte) string {
	var id []byte
	// Remove punctuation from header except '-' or '#'.
	// '\p{L}\p{N}\p{M}' is the unicode equivalent of '\w', https://www.regular-expressions.info/unicode.html.
	punctuation := regexp.MustCompile(`[^\p{L}\p{N}\p{M}-# ]`)
	header = punctuation.ReplaceAll(header, []byte(""))
	headerText := bytes.TrimLeft(bytes.ToLower(header), "#")
	// If header is just punctuation it comes up empty, so it cannot be linked.
	if len(headerText) <= 1 {
		return ""
	}

	for _, h := range headerText[1:] {
		switch h {
		case '{':
			return string(id)
		case ' ', '-':
			id = append(id, '-')
		default:
			id = append(id, h)
		}
	}
	return string(id)
}

func absLocalLink(anchorDir string, docPath string, destination string) string {
	newDest := destination
	switch {
	case filepath.IsAbs(destination):
		return filepath.Join(anchorDir, destination[1:])
	case destination == ".":
		newDest = filepath.Base(docPath)
	case strings.HasPrefix(destination, "#"):
		newDest = filepath.Base(docPath) + destination
	case strings.Contains(destination, "/#"):
		destination = strings.Replace(destination, "/#", "#", 1)
		return filepath.Join(anchorDir, destination)
	}
	return filepath.Join(filepath.Dir(docPath), newDest)
}

func absLinkToRelLink(absLink string, docPath string) ([]byte, error) {
	absLinkSplit := strings.Split(absLink, "#")
	rel, err := filepath.Rel(filepath.Dir(docPath), absLinkSplit[0])
	if err != nil {
		return nil, err
	}

	if rel == filepath.Base(docPath) {
		rel = "."
	}

	if len(absLinkSplit) == 1 {
		return []byte(rel), nil
	}

	if rel != "." {
		return append([]byte(rel), append([]byte{'#'}, absLinkSplit[1]...)...), nil
	}
	return append([]byte{'#'}, absLinkSplit[1]...), nil
}
