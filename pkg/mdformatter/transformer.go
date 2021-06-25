// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package mdformatter

import (
	"bytes"
	"io"
	"regexp"
	"strconv"

	"github.com/efficientgo/tools/core/pkg/merrors"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"golang.org/x/net/html"
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

	sourceCtx SourceContext

	link           LinkTransformer
	cb             CodeBlockTransformer
	frontMatterLen int
}

func (t *transformer) Render(w io.Writer, source []byte, node ast.Node) error {
	if t.link == nil && t.cb == nil {
		return t.wrapped.Render(w, source, node)
	}

	if err := ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		var err error
		switch typedNode := n.(type) {
		case *ast.HTMLBlock, *ast.RawHTML:
			if !entering || t.link == nil {
				return ast.WalkSkipChildren, nil
			}

			// Parse HTML to get inline links on our own, goldmark does not do that.
			b := bytes.Buffer{}
			if typedNode, ok := n.(*ast.RawHTML); ok {
				for i := 0; i < typedNode.Segments.Len(); i++ {
					segment := typedNode.Segments.At(i)
					_, _ = b.Write(segment.Value(source))
				}
			} else {
				// We switch this to string type so we need to accommodate newlines.
				_, _ = b.WriteString("\n")
				if n.HasBlankPreviousLines() {
					_, _ = b.WriteString("\n")
				}
				for i := 0; i < n.Lines().Len(); i++ {
					segment := n.Lines().At(i)
					_, _ = b.Write(segment.Value(source))
				}
			}

			var out string
			z := html.NewTokenizer(&b)
			for tt := z.Next(); tt != html.ErrorToken; tt = z.Next() {
				token := z.Token()
				switch token.Data {
				case "img":
					for i := range token.Attr {
						if token.Attr[i].Key != "src" {
							continue
						}
						t.sourceCtx.LineNumbers = getLinkLines(source, []byte(token.Attr[i].Val), t.frontMatterLen)
						dest, err := t.link.TransformDestination(t.sourceCtx, []byte(token.Attr[i].Val))
						if err != nil {
							return ast.WalkStop, err
						}
						token.Attr[i].Val = string(dest)
						break
					}
				case "a":
					for i := range token.Attr {
						if token.Attr[i].Key != "href" {
							continue
						}
						t.sourceCtx.LineNumbers = getLinkLines(source, []byte(token.Attr[i].Val), t.frontMatterLen)
						dest, err := t.link.TransformDestination(t.sourceCtx, []byte(token.Attr[i].Val))
						if err != nil {
							return ast.WalkStop, err
						}
						token.Attr[i].Val = string(dest)
						break
					}
				}
				out += token.String()
			}
			if err := z.Err(); err != nil && err != io.EOF {
				return ast.WalkStop, err
			}

			repl := ast.NewString([]byte("\n" + out + "\n"))
			repl.SetParent(n.Parent())
			repl.SetPreviousSibling(n.PreviousSibling())
			repl.SetNextSibling(n.NextSibling())
			n.Parent().ReplaceChild(n.Parent(), n, repl)
			n.SetNextSibling(repl.NextSibling()) // Make sure our loop can continue.
		case *ast.Link:
			if !entering || t.link == nil {
				return ast.WalkSkipChildren, nil
			}
			t.sourceCtx.LineNumbers = getLinkLines(source, typedNode.Destination, t.frontMatterLen)
			typedNode.Destination, err = t.link.TransformDestination(t.sourceCtx, typedNode.Destination)
			if err != nil {
				return ast.WalkStop, err
			}
		case *ast.AutoLink:
			if !entering || t.link == nil || typedNode.AutoLinkType != ast.AutoLinkURL {
				return ast.WalkSkipChildren, nil
			}
			t.sourceCtx.LineNumbers = getLinkLines(source, typedNode.URL(source), t.frontMatterLen)
			dest, err := t.link.TransformDestination(t.sourceCtx, typedNode.URL(source))
			if err != nil {
				return ast.WalkStop, err
			}
			if bytes.Equal(dest, typedNode.URL(source)) {
				return ast.WalkSkipChildren, nil
			}
			repl := ast.NewString(dest)
			repl.SetParent(n)
			n.Parent().ReplaceChild(n.Parent(), n, repl)
			n.SetNextSibling(repl.NextSibling()) // Make sure our loop can continue.

		case *ast.Image:
			if !entering || t.link == nil {
				return ast.WalkSkipChildren, nil
			}
			t.sourceCtx.LineNumbers = getLinkLines(source, typedNode.Destination, t.frontMatterLen)
			typedNode.Destination, err = t.link.TransformDestination(t.sourceCtx, typedNode.Destination)
			if err != nil {
				return ast.WalkStop, err
			}
		case *ast.FencedCodeBlock:
			if !entering || t.cb == nil || typedNode.Info == nil {
				return ast.WalkSkipChildren, nil
			}
			blockContent, err := t.cb.TransformCodeBlock(t.sourceCtx, typedNode.Info.Text(source), typedNode.Text(source))
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

func (t *transformer) Close(ctx SourceContext) error {
	errs := merrors.New()
	if t.link != nil {
		errs.Add(t.link.Close(ctx))
	}
	if t.cb != nil {
		errs.Add(t.cb.Close(ctx))
	}
	return errs.Err()
}

func replaceContent(b *ast.BaseBlock, lastSegmentStop int, content []byte) {
	s := text.NewSegments()
	// NOTE(bwplotka): This feels like hack, because we pack all lines in single line. But it works (:
	s.Append(text.NewSegment(lastSegmentStop, lastSegmentStop+len(content)))
	b.SetLines(s)
}

// getLinkLines returns line numbers in source where link is present.
func getLinkLines(source []byte, link []byte, lenfm int) string {
	var targetLines string
	sourceLines := bytes.Split(source, []byte("\n"))
	// frontMatter is present so would need to account for `---` lines.
	if lenfm > 0 {
		lenfm += 2
	}
	// Using regex, as two links may have same host but diff params. Same in case of local links.
	linkRe := regexp.MustCompile(`(^|[^/\-~&=#?@%a-zA-Z0-9])` + string(link) + `($|[^/\-~&=#?@%a-zA-Z0-9])`)
	for i, line := range sourceLines {
		if linkRe.Match(line) {
			// Easier to just return int slice, but then cannot use it in futureKey.
			// https://golang.org/ref/spec#Map_types.
			add := strconv.Itoa(i + 1 + lenfm)
			if targetLines != "" {
				add = "," + strconv.Itoa(i+1+lenfm)
			}
			targetLines += add
		}
	}
	// If same link is found multiple times returns string like *,*,*...
	return targetLines
}
