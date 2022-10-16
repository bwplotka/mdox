// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package mdformatter

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/efficientgo/tools/core/pkg/testutil"
	"github.com/go-kit/kit/log"
)

func TestFormat_FormatSingle_NoTransformers(t *testing.T) {
	file, err := os.OpenFile("testdata/not_formatted.md", os.O_RDONLY, 0)
	testutil.Ok(t, err)
	defer file.Close()

	f := New(context.Background())

	exp, err := os.ReadFile("testdata/formatted.md")
	testutil.Ok(t, err)

	t.Run("Format not formatted", func(t *testing.T) {
		buf := bytes.Buffer{}
		testutil.Ok(t, f.Format(file, &buf))
		testutil.Equals(t, string(exp), buf.String())
	})

	t.Run("Format formatted", func(t *testing.T) {
		file2, err := os.OpenFile("testdata/formatted.md", os.O_RDONLY, 0)
		testutil.Ok(t, err)
		defer file2.Close()

		buf := bytes.Buffer{}
		testutil.Ok(t, f.Format(file2, &buf))
		testutil.Equals(t, string(exp), buf.String())
	})
}

func TestCheck_NoTransformers(t *testing.T) {
	diff, err := IsFormatted(context.Background(), log.NewNopLogger(), []string{"testdata/formatted.md"})
	testutil.Ok(t, err)
	testutil.Equals(t, 0, len(diff))
	testutil.Equals(t, "files the same; no diff", diff.String())

	diff, err = IsFormatted(context.Background(), log.NewNopLogger(), []string{"testdata/not_formatted.md"})
	testutil.Ok(t, err)

	exp, err := os.ReadFile("testdata/not_formatted.md.diff")
	testutil.Ok(t, err)
	testutil.Equals(t, string(exp), diff.String())
}

type mockLinkTransformer struct {
	closed bool
}

func (*mockLinkTransformer) TransformDestination(ctx SourceContext, destination []byte) ([]byte, error) {
	if bytes.HasPrefix(destination, []byte("$$-")) {
		return destination, nil
	}
	b := bytes.NewBufferString("$$-")
	_, _ = b.Write(destination)
	_, _ = b.WriteString("-")
	_, _ = b.WriteString(ctx.Filepath)
	_, _ = b.WriteString("-$$")
	return b.Bytes(), nil
}

func (m *mockLinkTransformer) Close(SourceContext) error {
	m.closed = true
	return nil
}

func TestFormat_FormatSingle_Transformers(t *testing.T) {
	file, err := os.OpenFile("testdata/not_formatted.md", os.O_RDONLY, 0)
	testutil.Ok(t, err)
	defer file.Close()

	m := &mockLinkTransformer{}
	f := New(context.Background())
	f.link = m

	exp, err := os.ReadFile("testdata/formatted_and_transformed.md")
	testutil.Ok(t, err)

	t.Run("Format not formatted", func(t *testing.T) {
		buf := bytes.Buffer{}
		testutil.Ok(t, f.Format(file, &buf))

		testutil.Equals(t, string(exp), buf.String())
	})

	t.Run("Format formatted", func(t *testing.T) {
		file2, err := os.OpenFile("testdata/formatted_and_transformed.md", os.O_RDONLY, 0)
		testutil.Ok(t, err)
		defer file2.Close()

		buf := bytes.Buffer{}
		testutil.Ok(t, f.Format(file2, &buf))
		testutil.Equals(t, string(exp), buf.String())
	})

	testutil.Equals(t, true, m.closed)
}

func TestFormat_FormatSingle_SoftWraps(t *testing.T) {
	file, err := os.OpenFile("testdata/not_formatted_softwraps.md", os.O_RDONLY, 0)
	testutil.Ok(t, err)
	defer file.Close()

	f := New(context.Background(), WithSoftWraps())

	exp, err := os.ReadFile("testdata/formatted_softwraps.md")
	testutil.Ok(t, err)

	t.Run("Format not formatted", func(t *testing.T) {
		buf := bytes.Buffer{}
		testutil.Ok(t, f.Format(file, &buf))
		testutil.Equals(t, string(exp), buf.String())
	})

	t.Run("Format formatted", func(t *testing.T) {
		file2, err := os.OpenFile("testdata/formatted_softwraps.md", os.O_RDONLY, 0)
		testutil.Ok(t, err)
		defer file2.Close()

		buf := bytes.Buffer{}
		testutil.Ok(t, f.Format(file2, &buf))
		testutil.Equals(t, string(exp), buf.String())
	})
}

func TestCheck_SoftWraps(t *testing.T) {
	diff, err := IsFormatted(context.Background(), log.NewNopLogger(), []string{"testdata/formatted_softwraps.md"}, WithSoftWraps())
	testutil.Ok(t, err)
	testutil.Equals(t, 0, len(diff))
	testutil.Equals(t, "files the same; no diff", diff.String())

	diff, err = IsFormatted(context.Background(), log.NewNopLogger(), []string{"testdata/not_formatted_softwraps.md"}, WithSoftWraps())
	testutil.Ok(t, err)

	exp, err := os.ReadFile("testdata/not_formatted_softwraps.md.diff")
	testutil.Ok(t, err)
	testutil.Equals(t, string(exp), diff.String())
}

func TestFormat_NoGofmt(t *testing.T) {
	file, err := os.OpenFile("testdata/not_formatted_nogofmt.md", os.O_RDONLY, 0)
	testutil.Ok(t, err)
	defer file.Close()

	f := New(context.Background(), WithNoCodeFmt())

	exp, err := os.ReadFile("testdata/not_formatted_nogofmt.md")
	testutil.Ok(t, err)

	buf := bytes.Buffer{}
	testutil.Ok(t, f.Format(file, &buf))
	testutil.Equals(t, string(exp), buf.String())
}
