package transform

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/gobwas/glob"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Version   int
	InputDir  string `yaml:"inputDir"`
	OutputDir string `yaml:"outputDir"`

	Transformations []*TransformationConfig

	// GitIgnore specifies if output dir should be git ignored or not.
	GitIgnore bool `yaml:"GitIgnore"`
}

type TransformationConfig struct {
	_glob glob.Glob

	// Glob matches files using https://github.com/gobwas/glob.
	// After first match, file is no longer matching other elements.
	Glob string

	// Skip skips moving matched files.
	Skip bool

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
	if err := yaml.Unmarshal(c, &cfg); err != nil {
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
