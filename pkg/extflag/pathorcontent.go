// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

// Taken from Thanos project.
//
// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.
package extflag

import (
	"fmt"
	"io/ioutil"
	"os"
	"regexp"

	"github.com/pkg/errors"
	"gopkg.in/alecthomas/kingpin.v2"
)

// PathOrContent is a flag type that defines two flags to fetch bytes. Either from file (*-file flag) or content (* flag).
type PathOrContent struct {
	flagName string

	envSubstitution bool
	required        bool

	path    *string
	content *string
}

// Option is a functional option type for PathOrContent objects.
type Option func(*PathOrContent)

type FlagClause interface {
	Flag(name, help string) *kingpin.FlagClause
}

// RegisterPathOrContent registers PathOrContent flag in kingpinCmdClause.
func RegisterPathOrContent(cmd FlagClause, flagName string, help string, opts ...Option) *PathOrContent {
	fileFlagName := fmt.Sprintf("%s-file", flagName)
	contentFlagName := flagName

	fileHelp := fmt.Sprintf("Path to %s", help)
	fileFlag := cmd.Flag(fileFlagName, fileHelp).PlaceHolder("<file-path>").String()

	contentHelp := fmt.Sprintf("Alternative to '%s' flag (mutually exclusive). Content of %s", fileFlagName, help)
	contentFlag := cmd.Flag(contentFlagName, contentHelp).PlaceHolder("<content>").String()

	p := &PathOrContent{
		flagName:        flagName,
		path:            fileFlag,
		content:         contentFlag,
		required:        false,
		envSubstitution: false,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Content returns the content of the file when given or directly the content that has been passed to the flag.
// It returns an error when:
// * The file and content flags are both not empty.
// * The file flag is not empty but the file can't be read.
// * The content is empty and the flag has been defined as required.
func (p *PathOrContent) Content() ([]byte, error) {
	fileFlagName := fmt.Sprintf("%s-file", p.flagName)

	if len(*p.path) > 0 && len(*p.content) > 0 {
		return nil, errors.Errorf("both %s and %s flags set.", fileFlagName, p.flagName)
	}

	var content []byte
	if len(*p.path) > 0 {
		c, err := ioutil.ReadFile(*p.path)
		if err != nil {
			return nil, errors.Wrapf(err, "loading file %s for %s", *p.path, fileFlagName)
		}
		content = c
	} else {
		content = []byte(*p.content)
	}

	if len(content) == 0 && p.required {
		return nil, errors.Errorf("flag %s or %s is required for running this command and content cannot be empty.", fileFlagName, p.flagName)
	}
	if p.envSubstitution {
		replace, err := expandEnv(content)
		if err != nil {
			return nil, err
		}
		content = replace
	}
	return content, nil
}

// WithRequired allows you to override default required option.
func WithRequired() Option {
	return func(p *PathOrContent) {
		p.required = true
	}
}

// WithRequired allows you to override default envSubstitution option.
func WithEnvSubstitution() Option {
	return func(p *PathOrContent) {
		p.envSubstitution = true
	}
}

// expandEnv returns content of YAML file with substituted environment variables.
// Follows K8s convention, i.e $(...), as mentioned here https://kubernetes.io/docs/tasks/inject-data-application/define-interdependent-environment-variables/.
func expandEnv(b []byte) (r []byte, err error) {
	var envRe = regexp.MustCompile(`\$\(([a-zA-Z_0-9]+)\)`)
	r = envRe.ReplaceAllFunc(b, func(n []byte) []byte {
		if err != nil {
			return nil
		}
		n = n[2 : len(n)-1]

		v, ok := os.LookupEnv(string(n))
		if !ok {
			err = errors.Errorf("found reference to unset environment variable %q", n)
			return nil
		}
		return []byte(v)
	})
	return r, err
}
