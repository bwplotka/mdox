// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package linktransformer

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bwplotka/mdox/pkg/mdformatter"
	"github.com/bwplotka/mdox/pkg/merrors"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/gocolly/colly/v2"
	"github.com/pkg/errors"
)

var remoteLinkPrefixRe = regexp.MustCompile(`^http[s]?://`)

type chain struct {
	chain []mdformatter.LinkTransformer
}

func NewChain(c ...mdformatter.LinkTransformer) mdformatter.LinkTransformer {
	return &chain{chain: c}
}

func (l *chain) TransformDestination(ctx context.Context, docPath string, destination []byte) (_ []byte, err error) {
	for _, c := range l.chain {
		destination, err = c.TransformDestination(ctx, docPath, destination)
		if err != nil {
			return nil, err
		}
	}
	return destination, nil
}

func (l *chain) Close() error {
	errs := merrors.New()
	for _, c := range l.chain {
		errs.Add(c.Close())
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

func (l *localizer) TransformDestination(_ context.Context, docPath string, destination []byte) (_ []byte, err error) {
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
		return absLinkToRelLink(newDest, docPath)
	}

	// Relative or absolute path.
	newDest := absLocalLink(l.anchorDir, docPath, string(destination))

	if err := l.localLinksByFile.Lookup(newDest); err != nil {
		level.Debug(l.logger).Log("msg", "attempted localization failed, no such local link; skipping", "err", err)
		return destination, nil
	}
	// NOTE: This assumes GetAnchorDir was used, so we validated if docPath is in the path of anchorDir.
	return absLinkToRelLink(newDest, docPath)
}

func (l *localizer) Close() error {
	return nil
}

type validator struct {
	localLinksByFile localLinksCache
	anchorDir        string

	except *regexp.Regexp
	c      *colly.Collector

	errs   *merrors.NilOrMultiError
	logger log.Logger
}

// NewValidator returns mdformatter.LinkTransformer that crawls all links.
func NewValidator(logger log.Logger, except *regexp.Regexp, anchorDir string) mdformatter.LinkTransformer {
	c := colly.NewCollector(colly.Async())
	errs := merrors.New()
	c.OnError(func(response *colly.Response, err error) {
		errs.Add(errors.Wrapf(err, "link %q; status code %v", response.Request.URL.String(), response.StatusCode))
	})
	return &validator{logger: logger, c: c, errs: errs, except: except, localLinksByFile: map[string]*[]string{}, anchorDir: anchorDir}
}

func (l *validator) TransformDestination(_ context.Context, docPath string, destination []byte) (_ []byte, err error) {
	if l.except.Match(destination) {
		return destination, nil
	}

	matches := remoteLinkPrefixRe.FindAllIndex(destination, 1)
	if matches == nil {
		// Relative or absolute path. Check if exists.
		newDest := absLocalLink(l.anchorDir, docPath, string(destination))

		// Local link. Check if exists.
		if err := l.localLinksByFile.Lookup(newDest); err != nil {
			l.errs.Add(errors.Wrapf(err, "link %v, normalized to %v", string(destination), newDest))
		}
		return destination, nil
	}

	// TODO(bwplotka): Respect context.
	if err := l.c.Visit(string(destination)); err != nil && err != colly.ErrAlreadyVisited {
		l.errs.Add(errors.Wrapf(err, "remote link %v", string(destination)))
	}
	return destination, nil
}

func (l *validator) Close() error {
	l.c.Wait()

	if err := l.errs.Err(); err != nil {
		for _, e := range err.Errors() {
			level.Warn(l.logger).Log("msg", e.Error())
		}
		return errors.Errorf("found %v problems with links.", len(err.Errors()))
	}
	return nil
}

type localLinksCache map[string]*[]string

type LookupError error

var (
	FileNotFoundErr = LookupError(errors.New("file not found"))
	IDNotFoundErr   = LookupError(errors.New("file exists, but does not have such id"))
)

func absLocalLink(anchorDir string, docPath string, destination string) string {
	newDest := destination
	switch {
	case filepath.IsAbs(destination):
		return filepath.Join(anchorDir, destination[1:])
	case destination == ".":
		newDest = filepath.Base(docPath)
	case strings.HasPrefix(destination, "#"):
		newDest = filepath.Base(docPath) + destination
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

// Lookup looks for given link in local anchorDir. It returns error if link can't be found.
func (l localLinksCache) Lookup(absLink string) error {
	absLinkSplit := strings.Split(absLink, "#")
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

func (l localLinksCache) addRelLinks(localFile string) error {
	// Add item for negative caching.
	l[localFile] = nil

	file, err := os.Open(localFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return errors.Wrapf(err, "failed to open file %v", localFile)
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
				return errors.Wrapf(err, "failed to read file %v", localFile)
			}
			break
		}

		if bytes.HasPrefix(b, []byte(`#`)) {
			ids = append(ids, toHeaderID(b))
		}
	}
	l[localFile] = &ids
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

// GetAnchorDir returns validated anchor dir against files provided.
func GetAnchorDir(anchorDir string, files []string) (_ string, err error) {
	if anchorDir == "" {
		anchorDir, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}

	// Check if provided files are within anchorDir way.
	for _, f := range files {
		if !strings.HasPrefix(f, anchorDir) {
			return "", errors.Errorf("anchorDir %q is not in path of provided file %q", anchorDir, f)
		}
	}
	return anchorDir, nil
}
