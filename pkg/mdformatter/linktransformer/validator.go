// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package linktransformer

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

const (
	gitHubAPIURL = "https://api.github.com/repos/%v/%v?sort=created&direction=desc&per_page=1"
)

type GitHubResponse struct {
	Number int `json:"number"`
}

// Match link with any one of provided validators.
func CheckValidators(dest string, v Config) bool {
	for _, val := range v.Validate.Validators {
		if val._regex.MatchString(dest) {
			if val.Type == "github" {
				// Find rightmost index of match i.e, where regex match ends.
				// This will be where issue/PR number starts. Split incase of section link and convert to int.
				idx := val._regex.FindStringIndex(dest)
				stringNum := strings.Split(dest[idx[1]:], "#")
				num, err := strconv.Atoi(stringNum[0])
				// If number in link does not exceed then link is valid. Otherwise will be checked by v.c.Visit.
				if val._maxnum >= num && err == nil {
					return true
				}
				return false
			}
			return true
		}
	}
	return false
}

// If type is "github", change regex and add maxnum.
func CheckGitHub(v Config) error {
	for i := range v.Validate.Validators {
		if v.Validate.Validators[i].Type == "github" {
			regex, maxnum, err := getGitHubRegex(v.Validate.Validators[i].Regex)
			if err != nil {
				return err
			}
			v.Validate.Validators[i]._regex = regex
			v.Validate.Validators[i]._maxnum = maxnum
		}
	}
	return nil
}

// Get GitHub pulls/issues regex from repo name.
func getGitHubRegex(repoRe string) (*regexp.Regexp, int, error) {
	if repoRe != "" {
		// Get reponame from regex.
		idx := strings.Index(repoRe, `\`)
		if idx == -1 {
			return nil, math.MaxInt64, errors.New("repo name regex not valid")
		}
		reponame := repoRe[:idx] + repoRe[idx+1:]

		var pullNum []GitHubResponse
		var issueNum []GitHubResponse
		max := 0
		// Check latest pull request number.
		respPull, err := http.Get(fmt.Sprintf(gitHubAPIURL, reponame, "pulls"))
		if err != nil {
			return nil, math.MaxInt64, err
		}
		if respPull.StatusCode != 200 {
			return nil, math.MaxInt64, errors.New("pulls API request failed. status code: " + strconv.Itoa(respPull.StatusCode))
		}
		defer respPull.Body.Close()
		if err := json.NewDecoder(respPull.Body).Decode(&pullNum); err != nil {
			return nil, math.MaxInt64, err
		}
		if len(pullNum) > 0 {
			max = pullNum[0].Number
		}

		// Check latest issue number and return whichever is greater.
		respIssue, err := http.Get(fmt.Sprintf(gitHubAPIURL, reponame, "issues"))
		if err != nil {
			return nil, math.MaxInt64, err
		}
		if respIssue.StatusCode != 200 {
			return nil, math.MaxInt64, errors.New("issues API request failed. status code: " + strconv.Itoa(respIssue.StatusCode))
		}
		defer respIssue.Body.Close()
		if err := json.NewDecoder(respIssue.Body).Decode(&issueNum); err != nil {
			return nil, math.MaxInt64, err
		}
		if len(issueNum) > 0 && issueNum[0].Number > max {
			max = issueNum[0].Number
		}

		return regexp.MustCompile(`(^http[s]?:\/\/)(www\.)?(github\.com\/)(` + repoRe + `)(\/pull\/|\/issues\/)`), max, nil
	}

	return regexp.MustCompile(`^$`), math.MaxInt64, nil
}
