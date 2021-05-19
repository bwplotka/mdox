// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package transform

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/gobwas/glob"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

type LinksStyle string

const (
	None LinksStyle = ""

	// Hugo make sure mdox converts the links to work on Hugo-like website so:
	// * Adds `slug: {{ FileName }}` to make sure filename extension is part of path, if slug is not added.
	// * Local links are lower cased (hugo does that by default).
	// * All links are expected to be paths e.g ../ is added to all local links.
	Hugo LinksStyle = "hugo"
)

type Config struct {
	Version int

	// InputDir is a relative path that assumes input directory for markdown files and assets.
	InputDir string `yaml:"inputDir"`
	// OutputDir is a relative output directory that we expect all files to land in. Typically that can be `content` dir
	// which hugo uses as an input.
	OutputDir string `yaml:"outputDir"`
	// OutputStaticDir is relative output directory for all non markdown files.
	OutputStaticDir string `yaml:"outputStaticDir"`

	// ExtraInputGlobs allows to bring files from outside of input dir.
	// NOTE: No one can link to this file from input dir.
	ExtraInputGlobs []string `yaml:"extraInputGlobs"`

	// Transformations to apply for any file with .md extension.
	Transformations []*TransformationConfig

	// GitIgnored specifies if output dir should be git ignored or not.
	GitIgnored bool `yaml:"gitIgnored"`

	// LocalLinksStyle sets linking style to be applied.
	LocalLinksStyle LinksStyle `yaml:"localLinksStyle"`
}

type TransformationConfig struct {
	_glob glob.Glob

	// Glob matches files using https://github.com/gobwas/glob.
	// After first match, file is no longer matching other elements.
	Glob string

	// Path is an optional different path for the file to be moved.
	// NOTE: All relative links will be moved accordingly.
	Path string

	// FrontMatter holds front matter transformations.
	FrontMatter *FrontMatterConfig `yaml:"frontMatter"`
}

type FrontMatterConfig struct {
	_template *template.Template

	// Template represents Go template that will be rendered as font matter.
	// This will override any existing font matter.1
	// TODO(bwplotka): Add add only option?
	Template string
}

func parseConfigFile(configFile string) (Config, error) {
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
		return Config{}, errors.Wrapf(err, "parsing template content %q", string(c))
	}

	if cfg.InputDir == "" {
		return Config{}, errors.New("contentDir field is required")
	}

	d, err := os.Stat(cfg.InputDir)
	if err != nil {
		return Config{}, err
	}
	if !d.IsDir() {
		return Config{}, errors.New("contentDir field is not pointing directory")
	}
	cfg.InputDir = strings.TrimSuffix(cfg.InputDir, "/")

	if cfg.OutputDir == "" {
		return Config{}, errors.New("outputDir field is required")
	}
	cfg.OutputDir = strings.TrimSuffix(cfg.OutputDir, "/")

	for _, f := range cfg.Transformations {
		f._glob, err = glob.Compile(f.Glob, '/')
		if err != nil {
			return Config{}, errors.Wrapf(err, "compiling glob %v", f.Glob)
		}

		if f.FrontMatter != nil {
			f.FrontMatter._template, err = template.New("").Parse(f.FrontMatter.Template)
			if err != nil {
				return Config{}, errors.Wrapf(err, "compiling template %v", f.FrontMatter.Template)
			}
		}
	}

	return cfg, nil
}
