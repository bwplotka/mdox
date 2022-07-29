package cache

import (
	"time"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

type cacheType string

const (
	timeDay              = 24 * time.Hour
	defaultCacheValidity = 5 * timeDay
	cacheTypeEmpty       = cacheType("")
	cacheTypeNone        = cacheType("none")
	cacheTypeSQLite      = cacheType("sqlite")
)

// Config holds the cache configuration.
type Config struct {
	// Type is the type of the cache.
	Type cacheType
	// Validity is the duration for which the cache is valid.
	Validity time.Duration
	// Jitter is the jitter to apply when considering a cached entry valid or not.
	Jitter time.Duration

	cacheParser *configParser
}

// NewConfig is the constructor for Config.
func NewConfig() Config {
	return Config{
		cacheParser: newConfigParser(),
	}
}

// Presents tell whether a cache configuration is present.
func (c *Config) Present() bool {
	return c.Type != cacheTypeNone
}

// UnmarshalYAML puts the unmarshaled yaml data into the internal cache parser
// struct. This prevents access to the string data of jitter and validity.
func (c *Config) UnmarshalYAML(value *yaml.Node) error {
	if err := value.Decode(c.cacheParser); err != nil {
		return err
	}
	if err := c.load(); err != nil {
		return err
	}
	return nil
}

// load validates the cache configuration from the parser and copy it
// into the configuration.
func (c *Config) load() error {
	switch c.cacheParser.Type {
	case cacheTypeSQLite:
		if c.cacheParser.Validity != "" {
			var err error
			c.Validity, err = time.ParseDuration(c.cacheParser.Validity)
			if err != nil {
				return errors.Wrap(err, "parsing cache validity duration")
			}
		}

		if c.cacheParser.Jitter != "" {
			var err error
			c.Jitter, err = time.ParseDuration(c.cacheParser.Jitter)
			if err != nil {
				return errors.Wrap(err, "parsing cache jitter duration")
			}
		}
	case cacheTypeNone, cacheTypeEmpty:
	default:
		return errors.New("unsupported cache type")
	}
	return nil
}

// configParser represents a cache configuration that can be parsed.
// These fields are not embed in a unified Config struct to avoid accidental
// usage of the duration fields (i.e. Validity and Jitter) as strings.
type configParser struct {
	Type     cacheType `yaml:"type"`
	Validity string    `yaml:"validity"`
	Jitter   string    `yaml:"jitter"`
}

// newConfigParser is the constructor for ConfigParser.
func newConfigParser() *configParser {
	return &configParser{
		Validity: defaultCacheValidity.String(),
	}
}
