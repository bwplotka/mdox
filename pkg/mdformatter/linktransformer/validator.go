// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package linktransformer

import (
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	whatwgurl "github.com/nlnwa/whatwg-url/url"
)

// Match Colly's normalization so requests, results, and cache entries share a key.
var remoteURLParser = whatwgurl.NewParser(whatwgurl.WithPercentEncodeSinglePercentSign())

func normalizeRemoteURL(dest string) (string, error) {
	u, err := remoteURLParser.Parse(dest)
	if err != nil {
		return "", err
	}
	// Colly also passes the WHATWG URL through net/url before issuing requests.
	parsed, err := url.Parse(u.Href(false))
	if err != nil {
		return "", err
	}
	return parsed.String(), nil
}

type Validator interface {
	IsValid(k futureKey, r *validator) (bool, error)
}

func (v LocalValidator) IsValid(k futureKey, r *validator) (bool, error) {
	r.l.localLinksChecked.Inc()
	// Check if link is email address.
	if email := strings.TrimPrefix(k.dest, "mailto:"); email != k.dest {
		if isValidEmail(email) {
			return true, nil
		}
		r.destFutures[k].resultFn = func() error { return fmt.Errorf("provided mailto link is not a valid email, got %v", k.dest) }
		return false, nil
	}

	anchorDir := r.anchorDir
	if v.anchor != "" {
		anchorDir = filepath.Join(anchorDir, v.anchor)
	}
	// Relative or absolute path. Check if exists.
	newDest := absLocalLink(anchorDir, k.filepath, k.dest)

	// Local link. Check if exists.
	if err := r.localLinks.Lookup(newDest); err != nil {
		r.destFutures[k].resultFn = func() error { return fmt.Errorf("link %v, normalized to: %w", k.dest, err) }
		return false, nil
	}
	return true, nil
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

// GitHubPullsIssuesValidator.IsValid skips visiting all GitHub issue/PR links.
func (v GitHubPullsIssuesValidator) IsValid(k futureKey, r *validator) (bool, error) {
	r.l.githubSkippedLinks.Inc()
	// Find rightmost index of match i.e, where regex match ends.
	// This will be where issue/PR number starts. Split incase of section link and convert to int.
	rightmostIndex := v._regex.FindStringIndex(k.dest)
	stringNum := strings.Split(k.dest[rightmostIndex[1]:], "#")
	num, err := strconv.Atoi(stringNum[0])
	if err != nil {
		return false, err
	}
	// If number in link does not exceed then link is valid.
	if v._maxNum >= num {
		return true, nil
	}
	return false, nil
}

// RoundTripValidator.IsValid returns true if URL is checked by colly or it is a valid local link.
func (v RoundTripValidator) IsValid(k futureKey, r *validator) (bool, error) {
	matches := remoteLinkPrefixRe.FindAllStringIndex(k.dest, 1)
	if matches == nil && r.validateConfig.ExplicitLocalValidators {
		r.l.localLinksChecked.Inc()
		return LocalValidator{}.IsValid(k, r)
	}

	dest, err := normalizeRemoteURL(k.dest)
	if err != nil {
		r.destFutures[k].resultFn = func() error { return fmt.Errorf("parse remote link %v: %w", k.dest, err) }
		return false, nil
	}
	// Result will be in future.
	r.destFutures[k].resultFn = func() error {
		r.rMu.RLock()
		defer r.rMu.RUnlock()
		return r.remoteLinks[dest]
	}
	r.rMu.RLock()
	if _, ok := r.remoteLinks[dest]; ok {
		r.rMu.RUnlock()
		return true, nil
	}
	r.rMu.RUnlock()

	r.rMu.Lock()
	defer r.rMu.Unlock()
	// We need to check again here to avoid race.
	if _, ok := r.remoteLinks[dest]; ok {
		return true, nil
	}

	if r.storage != nil {
		// Check if URL is already in cache database.
		if ok, err := r.storage.IsCached(dest); ok && err == nil {
			r.l.roundTripCachedLinks.Inc()
			r.remoteLinks[dest] = nil
			return true, nil
		}
	}

	r.l.roundTripVisitedLinks.Inc()
	// Reserve the result before scheduling so duplicates share in-flight retries.
	r.remoteLinks[dest] = nil
	if err := r.c.Visit(dest); err != nil {
		r.remoteLinks[dest] = fmt.Errorf("remote link %v: %w", k.dest, err)
		return false, nil
	}
	return true, nil
}

// IsValid returns true if matched so that link in not checked.
func (v IgnoreValidator) IsValid(k futureKey, r *validator) (bool, error) {
	r.l.ignoreSkippedLinks.Inc()

	return true, nil
}

// GetValidatorForURL returns correct Validator by matching URL.
func (v Config) GetValidatorForURL(URL string) Validator {
	for _, val := range v.Validators {
		switch val.Type {
		case roundtripValidator:
			if !val.rtValidator._regex.MatchString(URL) {
				continue
			}
			return val.rtValidator
		case githubPullsIssuesValidator:
			if !val.ghValidator._regex.MatchString(URL) {
				continue
			}
			return val.ghValidator
		case ignoreValidator:
			if !val.igValidator._regex.MatchString(URL) {
				continue
			}
			return val.igValidator
		case localValidator:
			if !val.lValidator._regex.MatchString(URL) {
				continue
			}
			return val.lValidator
		default:
			panic("unexpected validator type")
		}
	}
	// No config file passed, so all links must be checked.
	return RoundTripValidator{}
}
