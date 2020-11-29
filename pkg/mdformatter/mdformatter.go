// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package mdformatter

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"os"

	"github.com/Kunde21/markdownfmt/v2/markdown"
	"github.com/bwplotka/mdox/pkg/gitdiff"
	"github.com/bwplotka/mdox/pkg/merrors"
	"github.com/bwplotka/mdox/pkg/runutil"
	"github.com/go-kit/kit/log"
	"github.com/gohugoio/hugo/parser/pageparser"
	"github.com/pkg/errors"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
)

type FrontMatterTransformer interface {
	TransformFrontMatter(ctx context.Context, docPath string, frontMatter map[string]interface{}) ([]byte, error)
	io.Closer
}

type LinkTransformer interface {
	TransformDestination(ctx context.Context, docPath string, destination []byte) ([]byte, error)
	io.Closer
}

type CodeBlockTransformer interface {
	TransformCodeBlock(ctx context.Context, docPath string, infoString []byte, code []byte) ([]byte, error)
	io.Closer
}

type Formatter struct {
	ctx context.Context

	fm   FrontMatterTransformer
	link LinkTransformer
	cb   CodeBlockTransformer
}

// Option is a functional option type for Formatter objects.
type Option func(*Formatter)

// WithFrontMatterTransformer allows you to override the default FrontMatterTransformer.
func WithFrontMatterTransformer(fm FrontMatterTransformer) Option {
	return func(m *Formatter) {
		m.fm = fm
	}
}

// WithLinkTransformer allows you to override the default LinkTransformer.
func WithLinkTransformer(l LinkTransformer) Option {
	return func(m *Formatter) {
		m.link = l
	}
}

// WithMetaBlockTransformer allows you to override the default CodeBlockTransformer.
func WithCodeBlockTransformer(cb CodeBlockTransformer) Option {
	return func(m *Formatter) {
		m.cb = cb
	}
}

func New(ctx context.Context, opts ...Option) *Formatter {
	f := &Formatter{
		ctx: ctx,
		fm:  RemoveFrontMatter{},
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

type RemoveFrontMatter struct{}

func (RemoveFrontMatter) TransformFrontMatter(_ context.Context, _ string, frontMatter map[string]interface{}) ([]byte, error) {
	for k := range frontMatter {
		delete(frontMatter, k)
	}
	return nil, nil
}

func (RemoveFrontMatter) Close() error { return nil }

// Format formats given markdown files in-place. IsFormatted `With...` function to see what modifiers you can add.
func Format(ctx context.Context, logger log.Logger, files []string, opts ...Option) error {
	f := New(ctx, opts...)

	b := bytes.Buffer{}
	// TODO(bwplotka): Do Concurrently.

	errs := merrors.New()
	for _, fn := range files {
		errs.Add(func() error {
			file, err := os.OpenFile(fn, os.O_RDWR, 0)
			if err != nil {
				return errors.Wrapf(err, "open %v", fn)
			}
			defer runutil.CloseWithLogOnErr(logger, file, "close file %v", fn)

			b.Reset()
			if err := f.Format(file, &b); err != nil {
				return err
			}

			n, err := file.WriteAt(b.Bytes(), 0)
			if err != nil {
				return errors.Wrapf(err, "write %v", fn)
			}
			return file.Truncate(int64(n))
		}())
	}
	return errs.Err()
}

type Diffs []gitdiff.Diff

func (d Diffs) String() string {
	if len(d) == 0 {
		return "files the same; no diff"
	}

	b := bytes.Buffer{}
	for _, diff := range d {
		_, _ = b.Write(diff.ToCombinedFormat())
	}
	return b.String()
}

// IsFormatted tries to formats given markdown files and return Diff if files are not formatted.
// If diff is empty it means all files are formatted.
func IsFormatted(ctx context.Context, logger log.Logger, files []string, opts ...Option) (diffs Diffs, err error) {
	f := New(ctx, opts...)
	b := bytes.Buffer{}

	// TODO(bwplotka): Do Concurrently.
	errs := merrors.New()
	for _, fn := range files {
		errs.Add(func() error {
			file, err := os.OpenFile(fn, os.O_RDWR, 0)
			if err != nil {
				return errors.Wrapf(err, "open %v", fn)
			}
			defer runutil.CloseWithLogOnErr(logger, file, "close file %v", fn)

			b.Reset()
			if err := f.Format(file, &b); err != nil {
				return err
			}

			if _, err := file.Seek(0, 0); err != nil {
				return err
			}

			in, err := ioutil.ReadAll(file)
			if err != nil {
				return errors.Wrapf(err, "read all %v", fn)
			}

			if !bytes.Equal(in, b.Bytes()) {
				diffs = append(diffs, gitdiff.CompareBytes(in, fn, b.Bytes(), fn+" (formatted)"))
			}
			return nil
		}())
	}
	return diffs, errs.Err()
}

// Format writes formatted input file into out writer.
func (f *Formatter) Format(file *os.File, out io.Writer) error {
	t := &transformer{
		wrapped: markdown.NewRenderer(),
		f:       f,
		docPath: file.Name(),
	}
	gm1 := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
		),
		goldmark.WithParserOptions(
			parser.WithAttribute(), // Enable # headers {#custom-ids}.
			parser.WithHeadingAttribute(),
		),
		goldmark.WithParserOptions(),
		goldmark.WithRenderer(nopOpsRenderer{Renderer: t}),
	)

	gm2 := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
		),
		goldmark.WithParserOptions(
			parser.WithAttribute(), // Enable # headers {#custom-ids}.
			parser.WithHeadingAttribute(),
		),
		goldmark.WithParserOptions(),
		goldmark.WithRenderer(markdown.NewRenderer()),
	)

	b, err := ioutil.ReadAll(file)
	if err != nil {
		return errors.Wrapf(err, "read %v", file.Name())
	}
	content := b
	frontMatter := map[string]interface{}{}
	fm, err := pageparser.ParseFrontMatterAndContent(bytes.NewReader(b))
	if err == nil && len(fm.FrontMatter) > 0 {
		content = fm.Content
		frontMatter = fm.FrontMatter
	}

	if f.fm != nil {
		hdr, err := f.fm.TransformFrontMatter(f.ctx, file.Name(), frontMatter)
		if err != nil {
			return err
		}
		if _, err := out.Write(hdr); err != nil {
			return err
		}
		if err := f.fm.Close(); err != nil {
			return err
		}
	}

	// Hack: run Convert two times to ensure deterministic whitespace alignment.
	// This also immediately show transformers which are not working well together etc.
	tmp := bytes.Buffer{}
	if err := gm1.Convert(content, &tmp); err != nil {
		return errors.Wrapf(err, "first formatting phase for %v", file.Name())
	}
	if err := t.Close(); err != nil {
		return err
	}
	if err := gm2.Convert(tmp.Bytes(), out); err != nil {
		return errors.Wrapf(err, "second formatting phase for %v", file.Name())
	}
	return nil
}
