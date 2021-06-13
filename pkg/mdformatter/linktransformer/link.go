// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package linktransformer

import (
	"bufio"
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwplotka/mdox/pkg/mdformatter"
	"github.com/efficientgo/tools/core/pkg/merrors"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/gocolly/colly/v2"
	"github.com/pkg/errors"
)

var remoteLinkPrefixRe = regexp.MustCompile(`^http[s]?://`)

type LookupError error

var (
	FileNotFoundErr = LookupError(errors.New("file not found"))
	IDNotFoundErr   = LookupError(errors.New("file exists, but does not have such id"))
)

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

	futureMu    sync.Mutex
	destFutures map[futureKey]*futureResult
}

type futureKey struct {
	filepath, dest string
}

type futureResult struct {
	// function giving result, promised after colly.Wait.
	resultFn func() error
	cases    int
}

// NewValidator returns mdformatter.LinkTransformer that crawls all links.
// TODO(bwplotka): Add optimization and debug modes - this is the main source of latency and pain.
func NewValidator(logger log.Logger, linksValidateConfig []byte, anchorDir string) (mdformatter.LinkTransformer, error) {
	var err error
	config := Config{}
	if string(linksValidateConfig) != "" {
		config, err = ParseConfig(linksValidateConfig)
		if err != nil {
			return nil, err
		}
	}
	v := &validator{
		logger:         logger,
		anchorDir:      anchorDir,
		validateConfig: config,
		localLinks:     map[string]*[]string{},
		remoteLinks:    map[string]error{},
		c:              colly.NewCollector(colly.Async()),
		destFutures:    map[futureKey]*futureResult{},
	}
	// Set very soft limits.
	// E.g github has 50-5000 https://docs.github.com/en/free-pro-team@latest/rest/reference/rate-limit limit depending
	// on api (only search is below 100).
	if err := v.c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 100,
	}); err != nil {
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
		v.remoteLinks[response.Ctx.Get(originalURLKey)] = nil
	})
	v.c.OnError(func(response *colly.Response, err error) {
		v.rMu.Lock()
		defer v.rMu.Unlock()
		retriesStr := response.Ctx.Get(numberOfRetriesKey)
		retries, _ := strconv.Atoi(retriesStr)
		switch response.StatusCode {
		case http.StatusTooManyRequests:
			if retries <= 0 {
				var retryAfter int
				// Retry calls same methods as Visit and makes request with same options.
				// So retryKey is incremented here if onError is called again after Retry. By default retries once.
				response.Ctx.Put(numberOfRetriesKey, strconv.Itoa(retries+1))
				retryAfter, convErr := strconv.Atoi(response.Headers.Get("Retry-After"))
				if convErr != nil {
					retryAfter = 1
				}
				time.Sleep(time.Duration(retryAfter) * time.Second)

				if retryErr := response.Request.Retry(); retryErr != nil {
					v.remoteLinks[response.Ctx.Get(originalURLKey)] = errors.Wrapf(err, "remote link retry %v", response.Ctx.Get(originalURLKey))
					break
				}
				v.remoteLinks[response.Ctx.Get(originalURLKey)] = errors.Wrapf(err, "%q rate limited even after retry; status code %v", response.Request.URL.String(), response.StatusCode)
			}
		case http.StatusMovedPermanently, http.StatusTemporaryRedirect, http.StatusServiceUnavailable:
			if retries <= 0 {
				response.Ctx.Put(numberOfRetriesKey, strconv.Itoa(retries+1))

				if retryErr := response.Request.Retry(); retryErr != nil {
					v.remoteLinks[response.Ctx.Get(originalURLKey)] = errors.Wrapf(err, "remote link retry %v", response.Ctx.Get(originalURLKey))
					break
				}
				v.remoteLinks[response.Ctx.Get(originalURLKey)] = errors.Wrapf(err, "%q not accessible even after retry; status code %v", response.Request.URL.String(), response.StatusCode)
			}
		default:
			v.remoteLinks[response.Ctx.Get(originalURLKey)] = errors.Wrapf(err, "%q not accessible; status code %v", response.Request.URL.String(), response.StatusCode)
		}
	})
	return v, nil
}

// MustNewValidator returns mdformatter.LinkTransformer that crawls all links.
func MustNewValidator(logger log.Logger, linksValidateConfig []byte, anchorDir string) mdformatter.LinkTransformer {
	v, err := NewValidator(logger, linksValidateConfig, anchorDir)
	if err != nil {
		panic(err)
	}
	return v
}

func (v *validator) TransformDestination(ctx mdformatter.SourceContext, destination []byte) (_ []byte, err error) {
	v.visit(ctx.Filepath, string(destination))
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
				merr.Add(errors.Wrapf(err, "%v", path))
				continue
			}
			merr.Add(errors.Wrapf(err, "%v (%v occurrences)", path, f.cases))
		}
	}
	return merr.Err()
}

func (v *validator) visit(filepath string, dest string) {
	v.futureMu.Lock()
	defer v.futureMu.Unlock()
	k := futureKey{filepath: filepath, dest: dest}
	if _, ok := v.destFutures[k]; ok {
		v.destFutures[k].cases++
		return
	}
	v.destFutures[k] = &futureResult{cases: 1, resultFn: func() error { return nil }}
	matches := remoteLinkPrefixRe.FindAllStringIndex(dest, 1)
	if matches == nil {
		// Relative or absolute path. Check if exists.
		newDest := absLocalLink(v.anchorDir, filepath, dest)

		// Local link. Check if exists.
		if err := v.localLinks.Lookup(newDest); err != nil {
			v.destFutures[k].resultFn = func() error { return errors.Wrapf(err, "link %v, normalized to", dest) }
		}
		return
	}
	validator := v.validateConfig.GetValidatorForURL(dest)
	if validator != nil {
		matched, err := validator.IsValid(k, v)
		if matched && err == nil {
			return
		}
	}
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

	for _, h := range bytes.TrimLeft(bytes.ToLower(header), "#")[1:] {
		if (h >= 97 && h <= 122) || (h >= 48 && h <= 57) {
			id = append(id, h)
		}
		switch h {
		case '{':
			return string(id)
		case ' ':
			id = append(id, '-')
		default:
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
