// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package mdox

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"os"

	"github.com/Kunde21/markdownfmt/v2/markdown"
	"github.com/bwplotka/mdox/pkg/runutil"
	"github.com/go-kit/kit/log"
	"github.com/gohugoio/hugo/parser/pageparser"
	"github.com/pkg/errors"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

type FrontMatterTransformer interface {
	TransformFrontMatter(docPath string, frontMatter map[string]interface{}) ([]byte, error)
}

type LinkTransformer interface {
	TransformDestination(docPath string, destination []byte) ([]byte, error)
}

type MetaCodeBlockTransformer interface {
	TransformMetaCodeBlock(docPath string, destination []byte) ([]byte, error)
}

type format struct {
	fm   FrontMatterTransformer
	link LinkTransformer
}

// Option is a functional option type for format objects.
type Option func(*format)

// WithFrontMatterTransformer allows you to override the default FrontMatterTransformer.
func WithFrontMatterTransformer(fm FrontMatterTransformer) Option {
	return func(m *format) {
		m.fm = fm
	}
}

// WithLinkTransformer allows you to override the default LinkTransformer.
func WithLinkTransformer(l LinkTransformer) Option {
	return func(m *format) {
		m.link = l
	}
}

func newDefaultFormat() *format {
	return &format{
		fm: RemoveFrontMatter{},
	}
}

type RemoveFrontMatter struct{}

func (RemoveFrontMatter) TransformFrontMatter(_ string, frontMatter map[string]interface{}) ([]byte, error) {
	for k := range frontMatter {
		delete(frontMatter, k)
	}
	return nil, nil
}

// Format formats given markdown files in-place.
func Format(_ context.Context, logger log.Logger, files []string, opts ...Option) error {
	f := newDefaultFormat()
	for _, opt := range opts {
		opt(f)
	}

	// TODO(bwplotka): Do Concurrently.
	for _, fn := range files {
		file, err := os.OpenFile(fn, os.O_RDWR, 0)
		if err != nil {
			return errors.Wrapf(err, "read %v", fn)
		}
		defer runutil.ExhaustCloseWithLogOnErr(logger, file, "close file")

		if err := f.FormatSingle(file, file); err != nil {
			return err
		}
	}
	return nil
}

// FormatSingle formats file.
func (f *format) FormatSingle(file *os.File, out io.Writer) error {
	t := &transformer{
		f:       f,
		docPath: file.Name(),
	}
	gm := goldmark.New(
		goldmark.WithExtensions(
			extension.Table,
			extension.Strikethrough,
			extension.TaskList,
			extension.Linkify,
		),
		goldmark.WithParserOptions(
			parser.WithAttribute(), // Enable # headers {#custom-ids}.
			parser.WithASTTransformers(util.Prioritized(t, 10)),
		),
		goldmark.WithParserOptions(),
		goldmark.WithRenderer(nopOpsRenderer{markdown.NewRenderer()}),
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
		hdr, err := f.fm.TransformFrontMatter(file.Name(), frontMatter)
		if err != nil {
			return err
		}
		if _, err := out.Write(hdr); err != nil {
			return err
		}
	}

	if err := gm.Convert(content, out); err != nil {
		return errors.Wrapf(err, "format %v", f)
	}
	return t.Err()
}

type nopOpsRenderer struct {
	renderer.Renderer
}

func (nopOpsRenderer) AddOptions(...renderer.Option) {}

type transformer struct {
	f       *format
	docPath string

	err error
}

func (t *transformer) Transform(node *ast.Document, r text.Reader, c parser.Context) {
	if t.err != nil || t.f.link == nil {
		return
	}

	if err := ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		var err error
		l, ok := n.(*ast.Link)
		if !ok {
			return ast.WalkContinue, nil
		}

		if entering {
			l.Destination, err = t.f.link.TransformDestination(t.docPath, l.Destination)
			if err != nil {
				return ast.WalkStop, err
			}
		}

		return ast.WalkSkipChildren, nil
	}); err != nil {
		t.err = err
	}
	return
}

func (t *transformer) Err() error {
	return t.err
}
