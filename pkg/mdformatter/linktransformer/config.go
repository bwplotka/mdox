// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package linktransformer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"time"

<<<<<<< HEAD
	"github.com/bwplotka/mdox/pkg/cache"
	"github.com/pkg/errors"
=======
>>>>>>> b900e5ffcabb79b9c5ff764fd6bbb5b931315a0a
	"gopkg.in/yaml.v3"
)

type Config struct {
	Version int

	Cache cache.Config `yaml:"cache"`

	ExplicitLocalValidators bool              `yaml:"explicitLocalValidators"`
	Validators              []ValidatorConfig `yaml:"validators"`
	Timeout                 string            `yaml:"timeout"`
	Parallelism             int               `yaml:"parallelism"`
	// HostMaxConns has to be a pointer because a zero value means no limits
	// and we have to tell apart 0 from not-present configurations.
	HostMaxConns *int   `yaml:"host_max_conns"`
	RandomDelay  string `yaml:"random_delay"`

	timeout     time.Duration
	randomDelay time.Duration
}

type ValidatorConfig struct {
	// Regex for type of validator. For `githubPullsIssues` this is: (^http[s]?:\/\/)(www\.)?(github\.com\/){ORG_NAME}\/{REPO_NAME}(\/pull\/|\/issues\/).
	Regex string `yaml:"regex"`
	// By default type is `roundtrip`. Could be `githubPullsIssues` or `ignore`.
	Type ValidatorType `yaml:"type"`
	// GitHub repo token to avoid getting rate limited.
	Token string `yaml:"token"`

	ghValidator GitHubPullsIssuesValidator
	rtValidator RoundTripValidator
	igValidator IgnoreValidator
}

type RoundTripValidator struct {
	_regex *regexp.Regexp
}

type GitHubPullsIssuesValidator struct {
	_regex  *regexp.Regexp
	_maxNum int
}

type IgnoreValidator struct {
	_regex *regexp.Regexp
}

type ValidatorType string

const (
	roundtripValidator         ValidatorType = "roundtrip"
	githubPullsIssuesValidator ValidatorType = "githubPullsIssues"
	ignoreValidator            ValidatorType = "ignore"
)

const (
	gitHubAPIURL = "https://api.github.com/repos/%v/%v?sort=created&direction=desc&per_page=1"
)

type GitHubResponse struct {
	Number int `json:"number"`
}

func ParseConfig(c []byte) (Config, error) {
	cfg := Config{Cache: cache.NewConfig()}
	dec := yaml.NewDecoder(bytes.NewReader(c))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("parsing YAML content %q: %w", string(c), err)
	}

	if cfg.Timeout != "" {
		var err error
		cfg.timeout, err = time.ParseDuration(cfg.Timeout)
		if err != nil {
			return Config{}, fmt.Errorf("parsing timeout duration: %w", err)
		}
	}

	if cfg.RandomDelay != "" {
		var err error
		cfg.randomDelay, err = time.ParseDuration(cfg.RandomDelay)
		if err != nil {
			return Config{}, fmt.Errorf("parsing random delay duration: %w", err)
		}
	}

	if cfg.Parallelism < 0 {
		return Config{}, errors.New("parsing parallelism, has to be > 0")
	}

	switch cfg.Cache.Type {
	case sqlite:
		if cfg.Cache.Validity != "" {
			var err error
			cfg.Cache.validity, err = time.ParseDuration(cfg.Cache.Validity)
			if err != nil {
				return Config{}, fmt.Errorf("parsing cache validity duration: %w", err)
			}
		}

		if cfg.Cache.Jitter != "" {
			var err error
			cfg.Cache.jitter, err = time.ParseDuration(cfg.Cache.Jitter)
			if err != nil {
				return Config{}, fmt.Errorf("parsing cache jitter duration: %w", err)
			}
		}
	case none, "":
	default:
		return Config{}, errors.New("unsupported cache type")
	}

	if len(cfg.Validators) <= 0 {
		return cfg, nil
	}

	// Evaluate regex for given validators.
	for i := range cfg.Validators {
		switch cfg.Validators[i].Type {
		case roundtripValidator:
			cfg.Validators[i].rtValidator._regex = regexp.MustCompile(cfg.Validators[i].Regex)
		case githubPullsIssuesValidator:
			// Get maxNum from provided regex or fail.
			regex, maxNum, err := getGitHubRegex(cfg.Validators[i].Regex, cfg.Validators[i].Token)
			if err != nil {
				return Config{}, fmt.Errorf("parsing githubPullsIssues Regex: %w", err)
			}
			cfg.Validators[i].ghValidator._regex = regex
			cfg.Validators[i].ghValidator._maxNum = maxNum
		case ignoreValidator:
			cfg.Validators[i].igValidator._regex = regexp.MustCompile(cfg.Validators[i].Regex)
		default:
			return Config{}, errors.New("Validator type not supported")
		}
	}
	return cfg, nil
}

// getGitHubRegex returns GitHub pulls/issues regex from repo name.
func getGitHubRegex(pullsIssuesRe string, repoToken string) (*regexp.Regexp, int, error) {
	// Get reponame from Pulls & Issues regex. This also checks whether user provided regex is valid (inception again!).
	getRepo := regexp.MustCompile(`\(\^http\[s\]\?:\\\/\\\/\)\(www\\\.\)\?\(github\\\.com\\\/\)(?P<org>[A-Za-z0-9_.-]+)\\\/(?P<repo>[A-Za-z0-9_.-]+)\(\\\/pull\\\/\|\\\/issues\\\/\)`)
	match := getRepo.FindStringSubmatch(pullsIssuesRe)
	if len(match) != 3 {
		return nil, math.MaxInt64, errors.New(`GitHub PR/Issue regex not valid. Correct regex: (^http[s]?:\/\/)(www\.)?(github\.com\/){ORG_NAME}\/{REPO_NAME}(\/pull\/|\/issues\/)`)
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
		return nil, math.MaxInt64, fmt.Errorf("pulls API request failed. status code: %d", respPull.StatusCode)
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
		return nil, math.MaxInt64, fmt.Errorf("issues API request failed. status code: %d", respIssue.StatusCode)
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

	return regexp.MustCompile(pullsIssuesRe), max, nil
}
