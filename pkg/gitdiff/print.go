// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

// gitdiff is an adaptation of https://github.com/sourcegraph/go-diff/blob/master/diff/print.go that prints popular diffmatchpatch.Diff diffs.

package gitdiff

import (
	"bytes"
	"fmt"
	"strings"
	"unsafe"

	"github.com/sergi/go-diff/diffmatchpatch"
)

type Diff struct {
	diffs    []diffmatchpatch.Diff
	aFn, bFn string
}

func yoloString(b []byte) string {
	return *((*string)(unsafe.Pointer(&b)))
}

func CompareBytes(a []byte, aFn string, b []byte, bFn string) Diff {
	return Compare(yoloString(a), aFn, yoloString(b), bFn)
}

func Compare(a, aFn, b, bFn string) Diff {
	dmp := diffmatchpatch.New()
	return Diff{
		diffs: DiffLines(dmp.DiffMain(a, b, true)),
		aFn:   aFn,
		bFn:   bFn,
	}
}

// CombineIntoLines traverse through diff and creates separate per line diff for each prefix and suffix diff chunks.
// NOTE: This is useful to normalize output to git diff.
func DiffLines(diff []diffmatchpatch.Diff) (ret []diffmatchpatch.Diff) {
	// TODO(bwplotka):  Everything is per line now, but we could merge same op lines. But... whatever (:
	var rollingDelLine, rollingAddLine string
	for _, d := range diff {
		for i, line := range strings.Split(d.Text, "\n") {
			if i > 0 {
				switch d.Type {
				case diffmatchpatch.DiffEqual:
					if rollingAddLine == rollingDelLine {
						ret = append(ret, diffmatchpatch.Diff{Type: d.Type, Text: rollingAddLine})
						rollingAddLine, rollingDelLine = "", ""
						break
					}
					ret = append(ret, diffmatchpatch.Diff{Type: diffmatchpatch.DiffDelete, Text: rollingDelLine})
					ret = append(ret, diffmatchpatch.Diff{Type: diffmatchpatch.DiffInsert, Text: rollingAddLine})
					rollingAddLine, rollingDelLine = "", ""
				case diffmatchpatch.DiffInsert:
					ret = append(ret, diffmatchpatch.Diff{Type: d.Type, Text: rollingAddLine})
					rollingAddLine = ""
				case diffmatchpatch.DiffDelete:
					ret = append(ret, diffmatchpatch.Diff{Type: d.Type, Text: rollingDelLine})
					rollingDelLine = ""
				}
			}

			switch d.Type {
			case diffmatchpatch.DiffEqual:
				rollingDelLine += line
				rollingAddLine += line
			case diffmatchpatch.DiffInsert:
				rollingAddLine += line
			case diffmatchpatch.DiffDelete:
				rollingDelLine += line
			}
		}
	}

	if rollingAddLine == rollingDelLine {
		return append(ret, diffmatchpatch.Diff{Type: diffmatchpatch.DiffEqual, Text: rollingAddLine})
	}
	ret = append(ret, diffmatchpatch.Diff{Type: diffmatchpatch.DiffDelete, Text: rollingDelLine})
	ret = append(ret, diffmatchpatch.Diff{Type: diffmatchpatch.DiffInsert, Text: rollingAddLine})
	return ret
}

// ToCombinedFormat prints diff in git combined diff format, specified in https://git-scm.com/docs/diff-format#_combined_diff_format.
func (d Diff) ToCombinedFormat() []byte {
	const contextLines = 3

	var buf bytes.Buffer
	_, _ = fmt.Fprintln(&buf, "---", d.aFn)
	_, _ = fmt.Fprintln(&buf, "+++", d.bFn)
	_, _ = buf.Write(PrintDMPDiff(d.diffs, contextLines))
	return buf.Bytes()
}

type entry struct {
	preLines, postLines []string
	buf                 bytes.Buffer
	aRef, bRef          int
	adds, dels          int
}

func (e *entry) reset() {
	e.preLines = e.preLines[:0]
	e.postLines = e.postLines[:0]
	e.buf.Reset()
	e.aRef, e.bRef, e.adds, e.dels = 0, 0, 0, 0
}

func (e *entry) started() bool {
	return e.adds+e.dels > 0
}

func (e *entry) finish(w *bytes.Buffer, contextLines int) {
	if !e.started() {
		return
	}
	_, _ = fmt.Fprintf(w, "@@ -%d,%d +%d,%d @@\n", e.aRef, e.dels, e.bRef, e.adds)
	_, _ = w.Write(e.buf.Bytes())
	if contextLines > len(e.postLines) {
		contextLines = len(e.postLines)
	}
	writeLines(w, e.postLines[:contextLines], ' ')
	e.reset()
}

func (e *entry) addChange(lines []string, op diffmatchpatch.Operation) {
	if len(lines) == 0 {
		return
	}

	if len(e.preLines) > 0 {
		writeLines(&e.buf, e.preLines, ' ')
		e.preLines = e.preLines[:0]
	}
	// Post lines if any, no becomes "mid lines"
	if len(e.postLines) > 0 {
		writeLines(&e.buf, e.postLines, ' ')
		e.postLines = e.postLines[:0]
	}

	switch op {
	case diffmatchpatch.DiffInsert:
		e.adds += len(lines)
		writeLines(&e.buf, lines, '+')
	case diffmatchpatch.DiffDelete:
		e.dels += len(lines)
		writeLines(&e.buf, lines, '-')
	}
}

func writeLines(b *bytes.Buffer, lines []string, sign byte) {
	for _, l := range lines {
		if l != "" || sign != ' ' {
			_ = b.WriteByte(sign)
		}
		_, _ = b.WriteString(l)
		_ = b.WriteByte('\n')
	}
}

// PrintDMPDiff prints diffmatchpatch.Diff slice in git combined diff format, specified in https://git-scm.com/docs/diff-format#_combined_diff_format.
// It's caller responsibility to add extended header lines and git diff header if needed.
func PrintDMPDiff(diff []diffmatchpatch.Diff, contextLines int) []byte {
	var (
		buf bytes.Buffer
		e   = &entry{}

		aLinesCount, bLinesCount int
	)

	for _, d := range diff {
		lines := strings.Split(d.Text, "\n")
		switch d.Type {
		case diffmatchpatch.DiffEqual:
			aLinesCount += len(lines)
			bLinesCount += len(lines)
			if e.started() {
				// Take 2x context lines, so we are sure continue one entry if changes are within similar place.
				needed := 2*contextLines - len(e.postLines)
				if needed > len(lines) {
					needed = len(lines)
				}

				e.postLines = append(e.postLines, lines[:needed]...)
				if len(e.postLines) == 2*contextLines {
					e.finish(&buf, contextLines)
				}
			}

			if !e.started() {
				end := len(lines) - contextLines
				cut := 0
				if end < 0 {
					cut = len(e.preLines) + end
					if cut < 0 {
						cut = 0
					}
					end = 0
				}
				e.preLines = append(e.preLines[cut:], lines[end:]...)
				e.aRef = aLinesCount - len(e.preLines)
				e.bRef = bLinesCount - len(e.preLines)
			}
		case diffmatchpatch.DiffInsert:
			bLinesCount += len(lines)
			e.addChange(lines, d.Type)
		case diffmatchpatch.DiffDelete:
			aLinesCount += len(lines)
			e.addChange(lines, d.Type)
		}
	}

	if e.started() {
		e.finish(&buf, contextLines)
	}
	return buf.Bytes()
}
