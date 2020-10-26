// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package mdformatter

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"testing"

	"github.com/bwplotka/mdox/pkg/testutil"
)

func TestFormat_FormatSingle_NoTransformers(t *testing.T) {
	file, err := os.OpenFile("testdata/not_formatted.md", os.O_RDONLY, 0)
	testutil.Ok(t, err)
	defer file.Close()

	f := New(context.Background())

	exp, err := ioutil.ReadFile("testdata/formatted.md")
	testutil.Ok(t, err)

	t.Run("Formatter not formatted", func(t *testing.T) {
		buf := bytes.Buffer{}
		testutil.Ok(t, f.Format(file, &buf))
		testutil.Equals(t, string(exp), buf.String())
	})

	t.Run("Formatter formatted", func(t *testing.T) {
		file2, err := os.OpenFile("testdata/formatted.md", os.O_RDONLY, 0)
		testutil.Ok(t, err)
		defer file2.Close()

		buf := bytes.Buffer{}
		testutil.Ok(t, f.Format(file2, &buf))
		testutil.Equals(t, string(exp), buf.String())
	})
}

type mockLinkTransformer struct{}

func (mockLinkTransformer) TransformDestination(_ context.Context, docPath string, destination []byte) ([]byte, error) {
	if bytes.HasPrefix(destination, []byte("$$-")) {
		return destination, nil
	}
	b := bytes.NewBufferString("$$-")
	_, _ = b.Write(destination)
	_, _ = b.WriteString("-")
	_, _ = b.WriteString(docPath)
	_, _ = b.WriteString("-$$")
	return b.Bytes(), nil
}

func TestFormat_FormatSingle_Transformers(t *testing.T) {
	file, err := os.OpenFile("testdata/not_formatted.md", os.O_RDONLY, 0)
	testutil.Ok(t, err)
	defer file.Close()

	f := New(context.Background())
	f.link = mockLinkTransformer{}

	buf := bytes.Buffer{}
	testutil.Ok(t, f.Format(file, &buf))

	exp, err := ioutil.ReadFile("testdata/formatted_and_transformed.md")
	testutil.Ok(t, err)
	testutil.Equals(t, string(exp), buf.String())
}
