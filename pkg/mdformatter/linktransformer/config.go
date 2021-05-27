// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package linktransformer

import (
	"bytes"
	"io/ioutil"
	"path/filepath"
	"regexp"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Version int

	Validate struct {
		Validators []Validator `yaml:"validators"`
	}
}

type Validator struct {
	_regex  *regexp.Regexp
	_maxnum int
	// Regex for type github is reponame matcher, like `bwplotka\/mdox`.
	Regex string `yaml:"regex"`
	// By default type is `roundtrip`. Could be `github`.
	Type string `yaml:"type"`
}

func parseConfigFile(configFile string) (Config, error) {
	if configFile == "" {
		return Config{}, nil
	}
	configFile, err := filepath.Abs(configFile)
	if err != nil {
		return Config{}, errors.Wrap(err, "abs")
	}
	c, err := ioutil.ReadFile(configFile)
	if err != nil {
		return Config{}, errors.Wrap(err, "read config file")
	}
	return ParseConfig(c)
}

func ParseConfig(c []byte) (Config, error) {
	cfg := Config{}
	dec := yaml.NewDecoder(bytes.NewReader(c))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return Config{}, errors.Wrapf(err, "parsing YAML content %q", string(c))
	}

	if len(cfg.Validate.Validators) <= 0 {
		return Config{}, errors.New("No validator provided")
	}

	for i := range cfg.Validate.Validators {
		cfg.Validate.Validators[i]._regex = regexp.MustCompile(cfg.Validate.Validators[i].Regex)
	}
	return cfg, nil
}
