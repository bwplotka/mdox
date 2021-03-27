// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package mdgen

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/bwplotka/mdox/pkg/mdformatter"
	"github.com/mattn/go-shellwords"

	"github.com/pkg/errors"
)

const (
	infoStringKeyLang     = "mdox-gen-lang"
	infoStringKeyType     = "mdox-gen-type"
	infoStringKeyExec     = "mdox-gen-exec"
	infoStringKeyExitCode = "mdox-expect-exit-code"
	infoStringKeyFile     = "mdox-gen-file"
	infoStringKeyLines    = "mdox-gen-lines"
)

type genCodeBlockTransformer struct{}

func NewCodeBlockTransformer() *genCodeBlockTransformer {
	return &genCodeBlockTransformer{}
}

func (t *genCodeBlockTransformer) TransformCodeBlock(ctx mdformatter.SourceContext, infoString []byte, code []byte) ([]byte, error) {
	if len(infoString) == 0 {
		return code, nil
	}

	infoFiels, err := shellwords.NewParser().Parse(string(infoString))
	if err != nil {
		return nil, errors.Wrapf(err, "parsing info string %v", string(infoString))
	}
	infoStringAttr := map[string]string{}
	for i, field := range infoFiels {
		val := strings.Split(field, "=")
		if i == 0 && len(val) == 2 {
			return nil, errors.Errorf("missing language info in fenced code block. Got info string %q", string(infoString))
		}
		switch val[0] {
		case infoStringKeyExec, infoStringKeyExitCode:
			if len(val) != 2 {
				return nil, errors.Errorf("got %q without variable. Expected format is e.g ```yaml %q=<value2> %q=<value2>. Got info string %q", val[0], infoStringKeyExitCode, infoStringKeyExec, string(infoString))
			}
			infoStringAttr[val[0]] = val[1]
		case infoStringKeyLang, infoStringKeyType:
			if len(val) != 2 {
				return nil, errors.Errorf("got %q without variable. Expected format is e.g ```yaml %q=<value2> %q=<value2>. Got info string %q", val[0], infoStringKeyLang, infoStringKeyType, string(infoString))
			}
			infoStringAttr[val[0]] = val[1]
		case infoStringKeyFile, infoStringKeyLines:
			if len(val) != 2 {
				return nil, errors.Errorf("got %q without variable. Expected format is e.g ```yaml %q=<value2> %q=<value2>. Got info string %q", val[0], infoStringKeyFile, infoStringKeyLines, string(infoString))
			}
			infoStringAttr[val[0]] = val[1]
		}
	}

	if len(infoStringAttr) == 0 {
		// Code fence without mdox attributes.
		return code, nil
	}

	// Code fence with command.
	if execCmd, ok := infoStringAttr[infoStringKeyExec]; ok {
		if len(infoStringAttr) > 2 {
			return nil, errors.Errorf("got ambiguous attributes: %v. Expected format for %q is e.g ```text %q=<value> . Got info string %q", infoStringAttr, infoStringKeyExec, infoStringKeyExec, string(infoString))
		}
		execArgs, err := shellwords.NewParser().Parse(execCmd)
		if err != nil {
			return nil, errors.Wrapf(err, "parsing exec command %v", execCmd)
		}

		// Execute and render output.
		b := bytes.Buffer{}
		cmd := exec.CommandContext(ctx, execArgs[0], execArgs[1:]...)
		cmd.Stderr = &b
		cmd.Stdout = &b
		if err := cmd.Run(); err != nil {
			expectedCode, _ := strconv.Atoi(infoStringAttr[infoStringKeyExitCode])
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == expectedCode {
				return b.Bytes(), nil
			}
			return nil, errors.Wrapf(err, "run %v", execCmd)
		}
		return b.Bytes(), nil
	}

	// Code fence with config gen.
	lang, langOk := infoStringAttr[infoStringKeyLang]
	typePath, typOk := infoStringAttr[infoStringKeyType]
	if typOk || langOk {
		if typOk != langOk {
			return nil, errors.Errorf("got ambiguous attributes: %v. Expected is e.g ```yaml %q=<value> %q=go . Got info string %q", infoStringAttr, infoStringKeyType, infoStringKeyLang, string(infoString))
		}
		switch lang {
		case "go", "golang":
			return genGo(ctx, "", typePath)
		default:
			return nil, errors.Errorf("expected language a first element of info string got %q; Got info string %q", lang, string(infoString))
		}
	}

	// Code fence with file.
	fileToCopy, fileOk := infoStringAttr[infoStringKeyFile]
	linesToCopy, lineOk := infoStringAttr[infoStringKeyLines]
	if fileOk {
		file, err := os.Open(fileToCopy)
		if err != nil {
			return nil, errors.Errorf("file could not be opened, ensure file path: %q is correct. Got info string %q", infoStringKeyFile, string(infoString))
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		scanner.Split(bufio.ScanLines)
		var text []byte

		switch lineOk {
		case true:
			val := strings.Split(linesToCopy, "-")
			start, errStart := strconv.Atoi(val[0])
			if errStart != nil {
				return nil, errors.Errorf("start line value isn't a number: %q. Got info string %q", val[0], string(infoString))
			}
			end, errEnd := strconv.Atoi(val[1])
			if errEnd != nil {
				return nil, errors.Errorf("end line value isn't a number: %q. Got info string %q", val[1], string(infoString))
			}
			if start >= end {
				return nil, errors.Errorf("line number range isn't valid: %q-%q. Got info string %q", val[0], val[1], string(infoString))
			}
			line := 0

			for scanner.Scan() {
				if line >= start && line <= end {
					text = append(text, scanner.Bytes()...)
					text = append(text, "\n"...)
				}
				line++
			}
		case false:
			for scanner.Scan() {
				text = append(text, scanner.Bytes()...)
				text = append(text, "\n"...)
			}
		}
		return text, nil
	}

	panic("should never get here")
}

func (t *genCodeBlockTransformer) Close(ctx mdformatter.SourceContext) error { return nil }

func genGo(ctx context.Context, moduleRoot string, typePath string) ([]byte, error) {
	// TODO(bwplotka): To be done.
	return nil, nil
}
