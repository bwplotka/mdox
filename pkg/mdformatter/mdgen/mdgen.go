// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package mdgen

import (
	"bytes"
	"os/exec"
	"strconv"
	"strings"

	"github.com/bwplotka/mdox/pkg/mdformatter"
	"github.com/mattn/go-shellwords"

	"github.com/pkg/errors"
)

const (
	infoStringKeyExec     = "mdox-exec"
	infoStringKeyExitCode = "mdox-expect-exit-code"
)

var (
	newLineChar = []byte("\n")
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
		val := []string{field}
		if i := strings.Index(field, "="); i != -1 {
			val = []string{field[:i], field[i+1:]}
		}
		if i == 0 && len(val) == 2 {
			return nil, errors.Errorf("missing language info in fenced code block. Got info string %q", string(infoString))
		}
		switch val[0] {
		case infoStringKeyExec:
			if len(val) != 2 {
				return nil, errors.Errorf("got %q without variable. Expected format is e.g ```yaml %s=\"<value1>\" but got %s", val[0], infoStringKeyExec, string(infoString))
			}
			infoStringAttr[val[0]] = val[1]
		case infoStringKeyExitCode:
			if len(val) != 2 {
				return nil, errors.Errorf("got %q without variable. Expected format is e.g ```yaml %s=\"<value1>\" but got %s", val[0], infoStringKeyExitCode, string(infoString))
			}
			infoStringAttr[val[0]] = val[1]
		}
	}

	if len(infoStringAttr) == 0 {
		// Code fence without mdox attributes.
		return code, nil
	}

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
			return nil, errors.Wrapf(err, "run %v, out: %v", execCmd, b.String())
		}
		output := b.Bytes()
		// Add newline to output if not present.
		if !bytes.HasSuffix(output, newLineChar) {
			output = append(output, newLineChar...)
		}
		return output, nil
	}

	panic("should never get here")
}

func (t *genCodeBlockTransformer) Close(ctx mdformatter.SourceContext) error { return nil }
