// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package mdgen

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/bwplotka/mdox/pkg/mdformatter"
	"github.com/efficientgo/tools/pkg/testutil"
)

const (
	testDocWithCode = `package test

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
	`
)

func TestFormat_FormatSingle_CodeBlockTransformer(t *testing.T) {
	f := mdformatter.New(context.Background(), mdformatter.WithCodeBlockTransformer(NewCodeBlockTransformer()))

	exp, err := ioutil.ReadFile("testdata/mdgen_formatted.md")
	testutil.Ok(t, err)

	t.Run("Format not formatted", func(t *testing.T) {
		file, err := os.OpenFile("testdata/mdgen_not_formatted.md", os.O_RDONLY, 0)
		testutil.Ok(t, err)
		defer file.Close()

		buf := bytes.Buffer{}
		testutil.Ok(t, f.Format(file, &buf))
		testutil.Equals(t, string(exp), buf.String())
	})

	t.Run("Format formatted", func(t *testing.T) {
		file2, err := os.OpenFile("testdata/mdgen_formatted.md", os.O_RDONLY, 0)
		testutil.Ok(t, err)
		defer file2.Close()

		buf := bytes.Buffer{}
		testutil.Ok(t, f.Format(file2, &buf))
		testutil.Equals(t, string(exp), buf.String())
	})
}

func TestFormat_FormatFileGen_CodeBlockTransformer(t *testing.T) {
	f := mdformatter.New(context.Background(), mdformatter.WithCodeBlockTransformer(NewCodeBlockTransformer()))
	tmpDir, err := ioutil.TempDir("", "test-filegen")
	testutil.Ok(t, err)
	t.Cleanup(func() { testutil.Ok(t, os.RemoveAll(tmpDir)) })

	testDocWithFileGen := "```go mdox-gen-file=" + filepath.Join(tmpDir, "test.go") + " mdox-gen-lines=17:19\n```"
	testDocWithFileGenFormatted := "```go mdox-gen-file=" + filepath.Join(tmpDir, "test.go") + " mdox-gen-lines=17:19\n\tfunc yoloString(b []byte) string {\n\t\treturn *((*string)(unsafe.Pointer(&b)))\n\t}\n```\n"

	testutil.Ok(t, ioutil.WriteFile(filepath.Join(tmpDir, "doc.md"), []byte(testDocWithFileGen), os.ModePerm))
	testutil.Ok(t, ioutil.WriteFile(filepath.Join(tmpDir, "doc_formatted.md"), []byte(testDocWithFileGenFormatted), os.ModePerm))
	testutil.Ok(t, ioutil.WriteFile(filepath.Join(tmpDir, "test.go"), []byte(testDocWithCode), os.ModePerm))

	exp, err := ioutil.ReadFile(filepath.Join(tmpDir, "doc_formatted.md"))
	testutil.Ok(t, err)

	t.Run("Codegen should match by taking lines from file.", func(t *testing.T) {
		file, err := os.OpenFile(filepath.Join(tmpDir, "doc.md"), os.O_RDONLY, 0)
		testutil.Ok(t, err)
		defer file.Close()

		buf := bytes.Buffer{}
		testutil.Ok(t, f.Format(file, &buf))
		testutil.Equals(t, string(exp), buf.String())
	})
}
