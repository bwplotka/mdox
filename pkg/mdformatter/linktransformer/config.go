// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package linktransformer

import (
	"bytes"
	"regexp"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Version int

	Validators []Validator `yaml:"validators"`
}

type Validator struct {
	_regex  *regexp.Regexp
	_maxNum int
	// Regex for type github is reponame matcher, like `bwplotka\/mdox`.
	Regex string `yaml:"regex"`
	// By default type is `roundtrip`. Could be `github`.
	Type string `yaml:"type"`
}

func ParseConfig(c []byte) (Config, error) {
	if string(c) == "" {
		return Config{}, nil
	}
	cfg := Config{}
	dec := yaml.NewDecoder(bytes.NewReader(c))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return Config{}, errors.Wrapf(err, "parsing YAML content %q", string(c))
	}

	if len(cfg.Validators) <= 0 {
		return Config{}, errors.New("No validator provided")
	}

	for i := range cfg.Validators {
		cfg.Validators[i]._regex = regexp.MustCompile(cfg.Validators[i].Regex)
	}
	return cfg, nil
}
