// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	"github.com/gobwas/glob"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

const (
	gitHubAPIURL = "https://api.github.com/repos/%v/%v?sort=created&direction=desc&per_page=1"
)

type LocalLinksStyle struct {
	// Hugo make sure mdox converts the links to work on Hugo-like website so:
	// * Adds `slug: {{ FileName }}` to make sure filename extension is part of path, if slug is not added.
	// * Local links are lower cased (hugo does that by default).
	// * All links are expected to be paths e.g ../ is added if they target local, non directory links.
	Hugo *HugoLocalLinksStyle
}
type HugoLocalLinksStyle struct {
	// e.g for google/docsy it is "_index.md"
	IndexFileName string `yaml:"indexFileName"`
}

type Config struct {
	Version int

	// InputDir is a relative (to PWD) path that assumes input directory for markdown files and assets.
	InputDir string `yaml:"inputDir"`
	// OutputDir is a relative (to PWD) output directory that we expect all files to land in. Typically that can be `content` dir
	// which hugo uses as an input.
	OutputDir string `yaml:"outputDir"`

	// ExtraInputGlobs allows to bring files from outside of input dir.
	ExtraInputGlobs []string `yaml:"extraInputGlobs"`

	// Transformations to apply for any file.
	Transformations []*TransformationConfig

	// GitIgnored specifies if output dir should be git ignored or not.
	GitIgnored bool `yaml:"gitIgnored"`

	// LocalLinksStyle sets linking style to be applied. If empty, we assume default style.
	LocalLinksStyle LocalLinksStyle `yaml:"localLinksStyle"`

	// Validators set multiple composable validators for link checking.
	Validators []ValidatorConfig `yaml:"validators"`
}

type TransformationConfig struct {
	ParsedGlob glob.Glob

	// Glob matches files using https://github.com/gobwas/glob.
	// Glob is matched against the relative path of the file in the input directory in
	// relation to the input directory. For example:
	// InputDir: dir1, File found in dir1/a/b/c/file.md, the given glob will be matched
	// against a/b/c/file.md.
	// After first match, file is no longer matching other elements.
	Glob string

	// Path is an optional different path for the file to be moved.
	// If not specified, file will be moved to the exact same position as in input directory.
	// Use absolute path to point the absolute structure where `/` means output directory.
	// If relative path is used, it will start in the directory the file is in the input directory.
	// NOTE: All relative links will be moved accordingly inside such file.
	// TODO(bwplotka): Explain ** and * suffixes and ability to specify "invalid" paths like "/../".
	Path string

	// FrontMatter holds front matter transformations.
	FrontMatter *FrontMatterConfig `yaml:"frontMatter"`
}

type ValidatorConfig struct {
	// Regex for type github is reponame matcher, like `bwplotka\/mdox`.
	Regex string `yaml:"regex"`
	// By default type is `ignore`. Could be `github` or `roundtrip`.
	Type ValidatorType `yaml:"type"`
	// GitHub repo token to avoid getting rate limited.
	Token string `yaml:"token"`

	// Validators based on type.
	GhValidator GitHubValidator
	RtValidator RoundTripValidator
	IgValidator IgnoreValidator
}

type RoundTripValidator struct {
	Regex *regexp.Regexp
}

type GitHubValidator struct {
	Regex  *regexp.Regexp
	MaxNum int
}

type IgnoreValidator struct {
	Regex *regexp.Regexp
}

type ValidatorType string

const (
	RoundTrip ValidatorType = "roundtrip"
	GitHub    ValidatorType = "github"
	Ignore    ValidatorType = "ignore"
)

func (tr TransformationConfig) TargetRelPath(relPath string) (_ string, err error) {
	if tr.Path == "" {
		return relPath, nil
	}

	if dir, file := filepath.Split(strings.TrimSuffix(tr.Glob, filepath.Ext(tr.Glob))); file == "**" {
		relPath, err = filepath.Rel(dir, relPath)
		if err != nil {
			return "", err
		}
	}

	currDir, currFile := filepath.Split(relPath)
	targetDir, targetSuffix := filepath.Split(strings.TrimPrefix(tr.Path, "/"))

	if strings.HasSuffix(tr.Path, "/*") {
		targetSuffix = currFile
	} else if strings.HasSuffix(tr.Path, "/**") {
		if !filepath.IsAbs(tr.Path) {
			return "", errors.Errorf("path has to be absolute if suffix /** is used, got %v", tr.Path)
		}

		targetSuffix = relPath
		targetDir = filepath.Dir(strings.TrimPrefix(tr.Path, "/"))

	}

	if !filepath.IsAbs(tr.Path) {
		targetDir = filepath.Join(currDir, targetDir)
	}

	return filepath.Join(targetDir, targetSuffix), nil
}

type FrontMatterConfig struct {
	ParsedTemplate *template.Template

	// Template represents Go template that will be rendered as font matter.
	// This will override any existing font matter.1
	// TODO(bwplotka): Add add only option?
	Template string
}

// ParseTranformConfig returns Config for transform.
func ParseTransformConfig(c []byte) (Config, error) {
	cfg := Config{}
	dec := yaml.NewDecoder(bytes.NewReader(c))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return Config{}, errors.Wrapf(err, "parsing template content %q", string(c))
	}

	if cfg.InputDir == "" {
		return Config{}, errors.New("inputDir field is required")
	}

	d, err := os.Stat(cfg.InputDir)
	if err != nil {
		return Config{}, err
	}
	if !d.IsDir() {
		return Config{}, errors.New("inputDir field is not pointing to a directory")
	}
	cfg.InputDir, err = filepath.Abs(cfg.InputDir)
	if err != nil {
		return Config{}, err
	}
	cfg.InputDir = strings.TrimSuffix(cfg.InputDir, "/")

	if cfg.OutputDir == "" {
		return Config{}, errors.New("outputDir field is required")
	}
	cfg.OutputDir, err = filepath.Abs(cfg.OutputDir)
	if err != nil {
		return Config{}, err
	}
	cfg.OutputDir = strings.TrimSuffix(cfg.OutputDir, "/")

	for _, f := range cfg.Transformations {
		f.ParsedGlob, err = glob.Compile(f.Glob, '/')
		if err != nil {
			return Config{}, errors.Wrapf(err, "compiling glob %v", f.Glob)
		}

		if f.FrontMatter != nil {
			f.FrontMatter.ParsedTemplate, err = template.New("").Parse(f.FrontMatter.Template)
			if err != nil {
				return Config{}, errors.Wrapf(err, "compiling template %v", f.FrontMatter.Template)
			}
		}
	}

	return cfg, nil
}

// ParseValidateConfig returns Config for linktransformer.
func ParseValidateConfig(c []byte) (Config, error) {
	cfg := Config{}
	dec := yaml.NewDecoder(bytes.NewReader(c))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return Config{}, errors.Wrapf(err, "parsing YAML content %q", string(c))
	}

	if len(cfg.Validators) <= 0 {
		return Config{}, errors.New("No validator provided")
	}

	// Evaluate regex for given validators.
	for i := range cfg.Validators {
		switch cfg.Validators[i].Type {
		case RoundTrip:
			cfg.Validators[i].RtValidator.Regex = regexp.MustCompile(cfg.Validators[i].Regex)
		case GitHub:
			regex, maxNum, err := getGitHubRegex(cfg.Validators[i].Regex, cfg.Validators[i].Token)
			if err != nil {
				return Config{}, errors.Wrapf(err, "parsing GitHub Regex %v", err)
			}
			cfg.Validators[i].GhValidator.Regex = regex
			cfg.Validators[i].GhValidator.MaxNum = maxNum
		case Ignore:
			cfg.Validators[i].IgValidator.Regex = regexp.MustCompile(cfg.Validators[i].Regex)
		default:
			return Config{}, errors.New("Validator type not supported")
		}
	}
	return cfg, nil
}

type GitHubResponse struct {
	Number int `json:"number"`
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
