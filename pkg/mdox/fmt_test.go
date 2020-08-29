// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package mdox

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"
)

func Test_FormatSingle_NoTransformers(t *testing.T) {
	file, err := os.OpenFile("testdata/not_formatted.md", os.O_RDONLY, 0)
	testutil.Ok(t, err)
	defer file.Close()

	f := newDefaultFormat()

	buf := bytes.Buffer{}
	testutil.Ok(t, f.FormatSingle(file, &buf))
	//testutil.Ok(t, ioutil.WriteFile("test.md", buf.Bytes(), os.ModePerm))

	exp, err := ioutil.ReadFile("testdata/formatted.md")
	testutil.Ok(t, err)
	testutil.Equals(t, string(exp), buf.String())
}

type mockLinkTransformer struct{}

func (mockLinkTransformer) TransformDestination(docPath string, destination []byte) ([]byte, error) {
	b := bytes.NewBufferString("$$-")
	_, _ = b.Write(destination)
	_, _ = b.WriteString("-")
	_, _ = b.WriteString(docPath)
	_, _ = b.WriteString("-$$")
	return b.Bytes(), nil
}

func Test_FormatSingle_Transformers(t *testing.T) {
	file, err := os.OpenFile("testdata/not_formatted.md", os.O_RDONLY, 0)
	testutil.Ok(t, err)
	defer file.Close()

	f := newDefaultFormat()
	f.link = mockLinkTransformer{}

	buf := bytes.Buffer{}
	testutil.Ok(t, f.FormatSingle(file, &buf))
	//testutil.Ok(t, ioutil.WriteFile("test.md", buf.Bytes(), os.ModePerm))

	exp, err := ioutil.ReadFile("testdata/formatted_and_transformed.md")
	testutil.Ok(t, err)
	testutil.Equals(t, string(exp), buf.String())
}
