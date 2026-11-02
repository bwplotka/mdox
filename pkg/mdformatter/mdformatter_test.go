// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package mdformatter

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/efficientgo/core/testutil"
	"github.com/go-kit/log"
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

func TestFormat_ReferenceDefinitions(t *testing.T) {
	for _, tc := range []struct {
		name, input, expected string
	}{
		{
			name:     "link with title",
			input:    "[link][ref]\n\n[ref]: https://example.com \"title\"\n",
			expected: "[link](https://example.com \"title\")\n",
		},
		{
			name:     "image with title",
			input:    "![image][ref]\n\n[ref]: https://example.com \"title\"\n",
			expected: "![image](https://example.com \"title\")\n",
		},
		{
			name:     "shortcut and collapsed references",
			input:    "[ref] and [ref][]\n\n[ref]: https://example.com\n",
			expected: "[ref](https://example.com) and [ref](https://example.com)\n",
		},
		{
			name:     "adjacent definitions",
			input:    "[one][a] and [two][b]\n\n[a]: https://example.com\n[b]: https://example.com\n",
			expected: "[one](https://example.com) and [two](https://example.com)\n",
		},
		{
			name:     "definition in blockquote",
			input:    "> [link][ref]\n>\n> [ref]: https://example.com\n",
			expected: "> [link](https://example.com)\n",
		},
		{
			name:     "unused definitions",
			input:    "[unused]: https://example.com\n[another]: https://example.com/another\n\nText.\n",
			expected: "Text.\n",
		},
		{
			name:     "only definitions",
			input:    "[unused]: https://example.com\n",
			expected: "\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, transform := range []bool{false, true} {
				name := "without transformer"
				if transform {
					name = "with transformer"
				}
				t.Run(name, func(t *testing.T) {
					path := filepath.Join(t.TempDir(), "references.md")
					f := New(context.Background())
					expected := tc.expected
					if transform {
						f.link = &mockLinkTransformer{}
						expected = strings.ReplaceAll(expected, "https://example.com", "$$-https://example.com-"+path+"-$$")
					}

					input := tc.input
					for range 2 {
						testutil.Ok(t, os.WriteFile(path, []byte(input), 0600))
						file, err := os.Open(path)
						testutil.Ok(t, err)
						var output bytes.Buffer
						err = f.Format(file, &output)
						testutil.Ok(t, file.Close())
						testutil.Ok(t, err)
						testutil.Equals(t, expected, output.String())
						input = output.String()
					}
				})
			}
		})
	}
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

func TestFormatBadlyFormattedGoCode(t *testing.T) {
	t.Run("Format without WithCodeFmt", func(t *testing.T) {
		file, err := os.OpenFile("testdata/badly_formatted_go_code.md", os.O_RDONLY, 0)
		testutil.Ok(t, err)
		defer file.Close()

		f := New(context.Background())

		exp, err := os.ReadFile("testdata/badly_formatted_go_code.md")
		testutil.Ok(t, err)

		buf := bytes.Buffer{}
		testutil.Ok(t, f.Format(file, &buf))
		testutil.Equals(t, string(exp), buf.String())
	})

	t.Run("Format", func(t *testing.T) {
		file, err := os.OpenFile("testdata/badly_formatted_go_code.md", os.O_RDONLY, 0)
		testutil.Ok(t, err)
		defer file.Close()

		f := New(context.Background(), WithCodeFmt())

		exp, err := os.ReadFile("testdata/badly_formatted_go_code.reformatted.md")
		testutil.Ok(t, err)

		buf := bytes.Buffer{}
		testutil.Ok(t, f.Format(file, &buf))
		testutil.Equals(t, string(exp), buf.String())
	})
}
