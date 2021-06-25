// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package mdformatter

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"os"
	"sort"
	"time"

	"github.com/Kunde21/markdownfmt/v2/markdown"
	"github.com/bwplotka/mdox/pkg/gitdiff"
	"github.com/efficientgo/tools/core/pkg/logerrcapture"
	"github.com/efficientgo/tools/core/pkg/merrors"
	"github.com/go-kit/kit/log"
	"github.com/gohugoio/hugo/parser/pageparser"
	"github.com/mattn/go-isatty"
	"github.com/pkg/errors"
	"github.com/theckman/yacspin"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"gopkg.in/yaml.v3"
)

type SourceContext struct {
	context.Context

	Filepath string
}

type FrontMatterTransformer interface {
	TransformFrontMatter(ctx SourceContext, frontMatter map[string]interface{}) ([]byte, error)
	Close(ctx SourceContext) error
}

type LinkTransformer interface {
	TransformDestination(ctx SourceContext, destination []byte) ([]byte, error)
	Close(ctx SourceContext) error
}

type CodeBlockTransformer interface {
	TransformCodeBlock(ctx SourceContext, infoString []byte, code []byte) ([]byte, error)
	Close(ctx SourceContext) error
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

// WithCodeBlockTransformer allows you to override the default CodeBlockTransformer.
func WithCodeBlockTransformer(cb CodeBlockTransformer) Option {
	return func(m *Formatter) {
		m.cb = cb
	}
}

func New(ctx context.Context, opts ...Option) *Formatter {
	f := &Formatter{
		ctx: ctx,
		fm:  FormatFrontMatterTransformer{},
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

type RemoveFrontMatter struct{}

func (RemoveFrontMatter) TransformFrontMatter(_ SourceContext, _ map[string]interface{}) ([]byte, error) {
	return nil, nil
}

func (RemoveFrontMatter) Close() error { return nil }

type FormatFrontMatterTransformer struct{}

func (FormatFrontMatterTransformer) TransformFrontMatter(_ SourceContext, frontMatter map[string]interface{}) ([]byte, error) {
	return FormatFrontMatter(frontMatter)
}

func FormatFrontMatter(m map[string]interface{}) ([]byte, error) {
	if len(m) == 0 {
		return nil, nil
	}

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(keys)))

	f := sortedFrontMatter{
		m:    m,
		keys: keys,
	}

	b := bytes.NewBuffer([]byte("---\n"))
	o, err := yaml.Marshal(f)
	if err != nil {
		return nil, errors.Wrap(err, "marshall front matter")
	}
	_, _ = b.Write(o)
	_, _ = b.Write([]byte("---\n\n"))
	return b.Bytes(), nil
}

var _ yaml.Marshaler = sortedFrontMatter{}

type sortedFrontMatter struct {
	m    map[string]interface{}
	keys []string
}

func (f sortedFrontMatter) MarshalYAML() (interface{}, error) {
	n := &yaml.Node{
		Kind: yaml.MappingNode,
	}

	for _, k := range f.keys {
		n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: k})

		b, err := yaml.Marshal(f.m[k])
		if err != nil {
			return nil, errors.Wrap(err, "map marshal")
		}
		v := &yaml.Node{}
		if err := yaml.Unmarshal(b, v); err != nil {
			return nil, err
		}

		// We expect a node of type document with single content containing other nodes.
		if len(v.Content) != 1 {
			return nil, errors.Errorf("unexpected node after unmarshalling interface: %#v", v)
		}
		// TODO(bwplotka): This creates weird indentation, fix it.
		n.Content = append(n.Content, v.Content[0])
	}
	return n, nil
}

func (FormatFrontMatterTransformer) Close(SourceContext) error { return nil }

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

func newSpinner(suffix string) (*yacspin.Spinner, error) {
	cfg := yacspin.Config{
		Frequency: 100 * time.Millisecond,
		CharSet:   yacspin.CharSets[11],
		Suffix:    suffix,
		ColorAll:  true,
		Writer:    os.Stderr,
		Colors:    []string{"cyan", "bold"},
	}

	spin, err := yacspin.New(cfg)
	if err != nil {
		return nil, err
	}
	if isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd()) {
		return spin, nil
	}
	return nil, nil
}

// Format formats given markdown files in-place. IsFormatted `With...` function to see what modifiers you can add.
func Format(ctx context.Context, logger log.Logger, files []string, opts ...Option) error {
	spin, err := newSpinner(" Formatting: ")
	if err != nil {
		return err
	}
	return format(ctx, logger, files, nil, spin, opts...)
}

// IsFormatted tries to formats given markdown files and return Diff if files are not formatted.
// If diff is empty it means all files are formatted.
func IsFormatted(ctx context.Context, logger log.Logger, files []string, opts ...Option) (diffs Diffs, err error) {
	d := Diffs{}
	spin, err := newSpinner(" Checking: ")
	if err != nil {
		return nil, err
	}
	if err := format(ctx, logger, files, &d, spin, opts...); err != nil {
		return nil, err
	}
	return d, nil
}

func format(ctx context.Context, logger log.Logger, files []string, diffs *Diffs, spin *yacspin.Spinner, opts ...Option) error {
	f := New(ctx, opts...)
	b := bytes.Buffer{}
	// TODO(bwplotka): Add concurrency (collector will need to redone).

	errs := merrors.New()
	if spin != nil {
		errs.Add(spin.Start())
	}
	for _, fn := range files {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if spin != nil {
			spin.Message(fn + "...")
		}
		errs.Add(func() error {
			file, err := os.OpenFile(fn, os.O_RDWR, 0)
			if err != nil {
				return errors.Wrapf(err, "open %v", fn)
			}
			defer logerrcapture.ExhaustClose(logger, file, "close file %v", fn)

			b.Reset()
			if err := f.Format(file, &b); err != nil {
				return err
			}

			if diffs != nil {
				if _, err := file.Seek(0, 0); err != nil {
					return err
				}

				in, err := ioutil.ReadAll(file)
				if err != nil {
					return errors.Wrapf(err, "read all %v", fn)
				}

				if !bytes.Equal(in, b.Bytes()) {
					*diffs = append(*diffs, gitdiff.CompareBytes(in, fn, b.Bytes(), fn+" (formatted)"))
				}
				return nil
			}

			n, err := file.WriteAt(b.Bytes(), 0)
			if err != nil {
				return errors.Wrapf(err, "write %v", fn)
			}
			return file.Truncate(int64(n))
		}())
	}
	if spin != nil {
		errs.Add(spin.Stop())
	}
	return errs.Err()
}

// Format writes formatted input file into out writer.
func (f *Formatter) Format(file *os.File, out io.Writer) error {
	sourceCtx := SourceContext{
		Context:  f.ctx,
		Filepath: file.Name(),
	}

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
		// TODO(bwplotka): Handle some front matter, wrongly put not as header.
		hdr, err := f.fm.TransformFrontMatter(sourceCtx, frontMatter)
		if err != nil {
			return err
		}
		if _, err := out.Write(hdr); err != nil {
			return err
		}
		if err := f.fm.Close(sourceCtx); err != nil {
			return err
		}
	}

	// Hack: run Convert two times to ensure deterministic whitespace alignment.
	// This also immediately show transformers which are not working well together etc.
	tmp := bytes.Buffer{}
	tr := &transformer{
		wrapped:   markdown.NewRenderer(),
		sourceCtx: sourceCtx,
		link:      f.link, cb: f.cb,
	}
	if err := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(parser.WithAttribute() /* Enable # headers {#custom-ids} */, parser.WithHeadingAttribute()),
		goldmark.WithRenderer(nopOpsRenderer{Renderer: tr}),
	).Convert(content, &tmp); err != nil {
		return errors.Wrapf(err, "first formatting phase for %v", file.Name())
	}
	if err := tr.Close(sourceCtx); err != nil {
		return errors.Wrapf(err, "%v", file.Name())
	}
	if err := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(parser.WithAttribute() /* Enable # headers {#custom-ids} */, parser.WithHeadingAttribute()),
		goldmark.WithRenderer(markdown.NewRenderer()), // No transforming for second phase.
	).Convert(tmp.Bytes(), out); err != nil {
		return errors.Wrapf(err, "second formatting phase for %v", file.Name())
	}
	return nil
}
