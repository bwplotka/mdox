// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package linktransformer

import (
	"strconv"
	"strings"
)

type Validator interface {
	IsValid(URL string) (bool, error)
}

// GitHubValidator.IsValid skips visiting all github issue/PR links.
func (v GitHubValidator) IsValid(URL string) (bool, error) {
	// Find rightmost index of match i.e, where regex match ends.
	// This will be where issue/PR number starts. Split incase of section link and convert to int.
	rightmostIndex := v._regex.FindStringIndex(URL)
	stringNum := strings.Split(URL[rightmostIndex[1]:], "#")
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

// RoundTripValidator.IsValid returns false if url matches, to ensure it is visited by colly.
func (v RoundTripValidator) IsValid(URL string) (bool, error) {
	return false, nil
}

// IgnoreValidator.IsValid returns true if matched so that link in not checked.
func (v IgnoreValidator) IsValid(URL string) (bool, error) {
	return true, nil
}

// GetValidatorForURL returns correct Validator by matching URL.
func (v Config) GetValidatorForURL(URL string) Validator {
	var u Validator
	for _, val := range v.Validators {
		switch val.Type {
		case roundtripValidator:
			if !val.rtValidator._regex.MatchString(URL) {
				continue
			}
			u = val.rtValidator
			return u
		case githubValidator:
			if !val.ghValidator._regex.MatchString(URL) {
				continue
			}
			u = val.ghValidator
			return u
		case ignoreValidator:
			if !val.igValidator._regex.MatchString(URL) {
				continue
			}
			u = val.igValidator
			return u
		default:
			continue
		}
	}
	// By default all links are ignored.
	u = IgnoreValidator{}
	// No config file passed, so all links must be checked.
	if len(v.Validators) == 0 {
		u = nil
	}
	return u
}
