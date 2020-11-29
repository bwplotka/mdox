// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package mdformatter

import (
	"bytes"
	"io"

	"github.com/bwplotka/mdox/pkg/merrors"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
)

type nopOpsRenderer struct {
	renderer.Renderer
}

func (nopOpsRenderer) AddOptions(...renderer.Option) {}

// transformer is a Renderer that is used as a transform layer just before render stage.
// This is to allow customer transforming that goes out of scope of goldmark AST Transformer.
type transformer struct {
	nopOpsRenderer

	wrapped renderer.Renderer

	f       *Formatter
	docPath string
}

func (t *transformer) Render(w io.Writer, source []byte, node ast.Node) error {
	if t.f.link == nil && t.f.cb == nil {
		return t.wrapped.Render(w, source, node)
	}

	if err := ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		var err error
		switch typedNode := n.(type) {
		case *ast.Link:
			if !entering || t.f.link == nil {
				return ast.WalkSkipChildren, nil
			}
			typedNode.Destination, err = t.f.link.TransformDestination(t.f.ctx, t.docPath, typedNode.Destination)
			if err != nil {
				return ast.WalkStop, err
			}
		case *ast.AutoLink:
			if !entering || t.f.link == nil || typedNode.AutoLinkType != ast.AutoLinkURL {
				return ast.WalkSkipChildren, nil
			}
			dest, err := t.f.link.TransformDestination(t.f.ctx, t.docPath, typedNode.URL(source))
			if err != nil {
				return ast.WalkStop, err
			}
			if bytes.Equal(dest, typedNode.URL(source)) {
				return ast.WalkSkipChildren, nil
			}
			repl := ast.NewString(dest)
			repl.SetParent(n)
			n.Parent().ReplaceChild(n.Parent(), n, repl)
		case *ast.FencedCodeBlock:
			if !entering || t.f.cb == nil || typedNode.Info == nil {
				return ast.WalkSkipChildren, nil
			}
			blockContent, err := t.f.cb.TransformCodeBlock(t.f.ctx, t.docPath, typedNode.Info.Text(source), typedNode.Text(source))
			if err != nil {
				return ast.WalkStop, err
			}
			if blockContent != nil {
				replaceContent(&typedNode.BaseBlock, len(source), blockContent)
				source = append(source, blockContent...)
			}
		default:
			return ast.WalkContinue, nil
		}
		return ast.WalkSkipChildren, nil
	}); err != nil {
		return err
	}
	return t.wrapped.Render(w, source, node)
}

func (t *transformer) Close() error {
	errs := merrors.New()
	if t.f.link != nil {
		errs.Add(t.f.link.Close())
	}
	if t.f.cb != nil {
		errs.Add(t.f.cb.Close())
	}
	return errs.Err()
}

func replaceContent(b *ast.BaseBlock, lastSegmentStop int, content []byte) {
	s := text.NewSegments()
	// NOTE(bwplotka): This feels like hack, because we pack all lines in single line. But it works (:
	s.Append(text.NewSegment(lastSegmentStop, lastSegmentStop+len(content)))
	b.SetLines(s)
}
