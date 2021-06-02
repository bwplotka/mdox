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

type URLValidator struct {
	Matched bool
}

// GetValidatorForURL matches link with any one of provided validators.
func (v Config) GetValidatorForURL(url string) URLValidator {
	u := URLValidator{Matched: false}
	for _, val := range v.Validators {
		if !val._regex.MatchString(url) {
			continue
		}
		if val.Type == roundtrip {
			u.Matched = true
			return u
		}
		// Find rightmost index of match i.e, where regex match ends.
		// This will be where issue/PR number starts. Split incase of section link and convert to int.
		rightmostIndex := val._regex.FindStringIndex(url)
		stringNum := strings.Split(url[rightmostIndex[1]:], "#")
		num, err := strconv.Atoi(stringNum[0])
		// If number in link does not exceed then link is valid. Otherwise will be checked by v.c.Visit.
		if val._maxNum >= num && err == nil {
			u.Matched = true
			return u
		}
	}
	return u
}

// validateGH changes regex and adds maxNum, if type is "github".
func (v Config) validateGH() error {
	for i := range v.Validators {
		if v.Validators[i].Type == roundtrip {
			continue
		}
		if v.Validators[i].Regex == "" {
			v.Validators[i]._regex = regexp.MustCompile(`^$`)
			v.Validators[i]._maxNum = math.MaxInt64
			continue
		}
		regex, maxNum, err := getGitHubRegex(v.Validators[i].Regex, v.Validators[i].Token)
		if err != nil {
			return err
		}
		v.Validators[i]._regex = regex
		v.Validators[i]._maxNum = maxNum
	}
	return nil
}

// getGitHubRegex returns GitHub pulls/issues regex from repo name.
func getGitHubRegex(repoRe string, repoToken string) (*regexp.Regexp, int, error) {
	// Get reponame from regex.
	getRepo := regexp.MustCompile(`(?P<org>[A-Za-z0-9_.-]+)\\\/(?P<repo>[A-Za-z0-9_.-]+)`)
	match := getRepo.FindStringSubmatch(repoRe)
	if len(match) != 3 {
		return nil, math.MaxInt64, errors.New("repo name regex not valid")
	}
	reponame := match[1] + "/" + match[2]

	var pullNum []GitHubResponse
	var issueNum []GitHubResponse
	max := 0
	// All GitHub API reqs need to have User-Agent: https://docs.github.com/en/rest/overview/resources-in-the-rest-api#user-agent-required.
	client := &http.Client{}

	// Check latest pull request number.
	reqPull, err := http.NewRequest("GET", fmt.Sprintf(gitHubAPIURL, reponame, "pulls"), nil)
	if err != nil {
		return nil, math.MaxInt64, err
	}
	reqPull.Header.Set("User-Agent", "mdox")

	// Check latest issue number and return whichever is greater.
	reqIssue, err := http.NewRequest("GET", fmt.Sprintf(gitHubAPIURL, reponame, "issues"), nil)
	if err != nil {
		return nil, math.MaxInt64, err
	}
	reqIssue.Header.Set("User-Agent", "mdox")

	if repoToken != "" {
		reqPull.Header.Set("Authorization", "Bearer "+repoToken)
		reqIssue.Header.Set("Authorization", "Bearer "+repoToken)
	}

	respPull, err := client.Do(reqPull)
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

	respIssue, err := client.Do(reqIssue)
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

	if len(pullNum) > 0 {
		max = pullNum[0].Number
	}
	if len(issueNum) > 0 && issueNum[0].Number > max {
		max = issueNum[0].Number
	}

	return regexp.MustCompile(`(^http[s]?:\/\/)(www\.)?(github\.com\/)(` + repoRe + `)(\/pull\/|\/issues\/)`), max, nil
}
