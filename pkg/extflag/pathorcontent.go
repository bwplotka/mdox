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
	"strings"

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

type FlagClause interface {
	Flag(name, help string) *kingpin.FlagClause
}

// RegisterPathOrContent registers PathOrContent flag in kingpinCmdClause.
func RegisterPathOrContent(cmd FlagClause, flagName string, help string, required bool, envSubstitution bool) *PathOrContent {
	fileFlagName := fmt.Sprintf("%s-file", flagName)
	contentFlagName := flagName

	fileHelp := fmt.Sprintf("Path to %s", help)
	fileFlag := cmd.Flag(fileFlagName, fileHelp).PlaceHolder("<file-path>").String()

	contentHelp := fmt.Sprintf("Alternative to '%s' flag (mutually exclusive). Content of %s", fileFlagName, help)
	contentFlag := cmd.Flag(contentFlagName, contentHelp).PlaceHolder("<content>").String()

	return &PathOrContent{
		flagName:        flagName,
		required:        required,
		path:            fileFlag,
		content:         contentFlag,
		envSubstitution: envSubstitution,
	}
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
		content = substituteEnvVars(string(content))
	}
	return content, nil
}

// substituteEnvVars returns content of YAML file with substituted environment variables.
// Will be substituted with empty string if env var isn't set.
// Follows K8s convention, i.e $(...), as mentioned here https://kubernetes.io/docs/tasks/inject-data-application/define-interdependent-environment-variables/.
func substituteEnvVars(content string) []byte {
	var replaceWithEnv []string
	// Match env variable syntax.
	envVarName := regexp.MustCompile(`\$\((?P<var>[a-zA-Z_]+[a-zA-Z0-9_]*)\)`)
	loc := envVarName.FindAllStringSubmatchIndex(content, -1)
	for i := range loc {
		// Add pair to be replaced.
		replaceWithEnv = append(replaceWithEnv, content[loc[i][0]:loc[i][1]], os.Getenv(content[loc[i][2]:loc[i][3]]))
	}
	replacer := strings.NewReplacer(replaceWithEnv...)
	contentWithEnv := replacer.Replace(content)
	return []byte(contentWithEnv)
}
