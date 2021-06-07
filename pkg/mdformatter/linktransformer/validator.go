// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package linktransformer

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bwplotka/mdox/pkg/config"
	"github.com/pkg/errors"
)

type Validator interface {
	IsValid(k futureKey, r *validator) (bool, error)
}

type RoundTripValidator config.RoundTripValidator

type GitHubValidator config.GitHubValidator

type IgnoreValidator config.IgnoreValidator

// GitHubValidator.IsValid skips visiting all github issue/PR links.
func (v GitHubValidator) IsValid(k futureKey, r *validator) (bool, error) {
	// Find rightmost index of match i.e, where regex match ends.
	// This will be where issue/PR number starts. Split incase of section link and convert to int.
	rightmostIndex := v.Regex.FindStringIndex(k.dest)
	stringNum := strings.Split(k.dest[rightmostIndex[1]:], "#")
	num, err := strconv.Atoi(stringNum[0])
	if err != nil {
		return false, err
	}
	// If number in link does not exceed then link is valid.
	if v.MaxNum >= num {
		return true, nil
	}
	return false, nil
}

// RoundTripValidator.IsValid returns true if url is checked by colly.
func (v RoundTripValidator) IsValid(k futureKey, r *validator) (bool, error) {
	// Result will be in future.
	r.destFutures[k].resultFn = func() error { return r.remoteLinks[k.dest] }
	r.rMu.RLock()
	if _, ok := r.remoteLinks[k.dest]; ok {
		r.rMu.RUnlock()
		return true, nil
	}
	r.rMu.RUnlock()

	r.rMu.Lock()
	defer r.rMu.Unlock()
	// We need to check again here to avoid race.
	if _, ok := r.remoteLinks[k.dest]; ok {
		return true, nil
	}

	if err := r.c.Visit(k.dest); err != nil {
		r.remoteLinks[k.dest] = errors.Wrapf(err, "remote link %v", k.dest)
		return false, nil
	}
	return true, nil
}

// IgnoreValidator.IsValid returns true if matched so that link in not checked.
func (v IgnoreValidator) IsValid(k futureKey, r *validator) (bool, error) {
	return true, nil
}

// GetValidatorForURL returns correct Validator by matching URL.
func GetValidatorForURL(URL string, v config.Config) Validator {
	for _, val := range v.Validators {
		switch val.Type {
		case config.RoundTrip:
			if !val.RtValidator.Regex.MatchString(URL) {
				continue
			}
			return RoundTripValidator(val.RtValidator)
		case config.GitHub:
			if !val.GhValidator.Regex.MatchString(URL) {
				continue
			}
			return GitHubValidator(val.GhValidator)
		case config.Ignore:
			if !val.IgValidator.Regex.MatchString(URL) {
				continue
			}
			return IgnoreValidator(val.IgValidator)
		default:
			panic(fmt.Sprintf("unexpected validator type %v", val.Type))
		}
	}
	// No config file passed, so all links must be checked.
	return RoundTripValidator{}
}
