// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package mdgen

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/bwplotka/mdox/pkg/mdformatter"
	"github.com/efficientgo/core/testutil"
)

func TestFormat_FormatSingle_CodeBlockTransformer(t *testing.T) {
	f := mdformatter.New(context.Background(), mdformatter.WithCodeBlockTransformer(NewCodeBlockTransformer()))

	exp, err := os.ReadFile("testdata/mdgen_formatted.md")
	testutil.Ok(t, err)

	t.Run("Format not formatted", func(t *testing.T) {
		file, err := os.OpenFile("testdata/mdgen_not_formatted.md", os.O_RDONLY, 0)
		testutil.Ok(t, err)
		defer file.Close()

		buf := bytes.Buffer{}
		testutil.Ok(t, f.Format(file, &buf))
		testutil.Equals(t, string(exp), buf.String())
	})

	t.Run("Format formatted", func(t *testing.T) {
		file2, err := os.OpenFile("testdata/mdgen_formatted.md", os.O_RDONLY, 0)
		testutil.Ok(t, err)
		defer file2.Close()

		buf := bytes.Buffer{}
		testutil.Ok(t, f.Format(file2, &buf))
		testutil.Equals(t, string(exp), buf.String())
	})
}
