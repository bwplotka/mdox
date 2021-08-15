// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package transform

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/gobwas/glob"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
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

	// GlueLink specifies link to be glued onto relative links which don't point to markdown or image files.
	GlueLink string `yaml:"glueLink"`

	// Transformations to apply for any file.
	Transformations []*TransformationConfig

	// GitIgnored specifies if output dir should be git ignored or not.
	GitIgnored bool `yaml:"gitIgnored"`

	// LocalLinksStyle sets linking style to be applied. If empty, we assume default style.
	LocalLinksStyle LocalLinksStyle `yaml:"localLinksStyle"`
}

type TransformationConfig struct {
	_glob glob.Glob

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

	PopHeader *bool `yaml:"popHeader"`

	// FrontMatter holds front matter transformations.
	FrontMatter *MatterConfig `yaml:"frontMatter"`

	// BackMatter holds back matter transformations.
	BackMatter *MatterConfig `yaml:"backMatter"`
}

func (tr TransformationConfig) targetRelPath(relPath string) (_ string, err error) {
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

type MatterConfig struct {
	_template *template.Template

	// Template represents Go template that will be rendered as matter.
	// This will override any existing matter.
	// TODO(bwplotka): Add add only option?
	Template string
}

func ParseConfig(c []byte) (Config, error) {
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
		f._glob, err = glob.Compile(f.Glob, '/')
		if err != nil {
			return Config{}, errors.Wrapf(err, "compiling glob %v", f.Glob)
		}

		if f.FrontMatter != nil {
			f.FrontMatter._template, err = template.New("").Parse(f.FrontMatter.Template)
			if err != nil {
				return Config{}, errors.Wrapf(err, "compiling frontMatter template %v", f.FrontMatter.Template)
			}
		}

		if f.BackMatter != nil {
			f.BackMatter._template, err = template.New("").Parse(f.BackMatter.Template)
			if err != nil {
				return Config{}, errors.Wrapf(err, "compiling backMatter template %v", f.BackMatter.Template)
			}
		}
	}

	return cfg, nil
}
