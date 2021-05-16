package web

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"text/template"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Version    int
	ContentDir string `yaml:"contentDir"`
	OutputDir  string `yaml:"outputDir"`

	FileMapping map[string]string             `yaml:"fileMapping"`
	FrontMatter map[string]*FrontMatterConfig `yaml:"frontMatter"`
}

type FrontMatterConfig struct {
	_r       *template.Template
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

	if cfg.ContentDir == "" {
		return Config{}, errors.New("contentDir field is required")
	}

	d, err := os.Stat(cfg.ContentDir)
	if err != nil {
		return Config{}, err
	}
	if !d.IsDir() {
		return Config{}, errors.New("contentDir field is not pointing directory")
	}

	if cfg.OutputDir == "" {
		return Config{}, errors.New("outputDir field is required")
	}

	for _, f := range cfg.FrontMatter {
		f._r, err = template.New("").Parse(f.Template)
		if err != nil {
			return Config{}, err
		}
	}

	return cfg, nil
}
