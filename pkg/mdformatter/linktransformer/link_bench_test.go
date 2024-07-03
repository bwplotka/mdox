// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package linktransformer

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/bwplotka/mdox/pkg/mdformatter"
	"github.com/efficientgo/core/testutil"
	"github.com/go-kit/log"
)

var testBuf bytes.Buffer

var (
	testDoc = `# Hey

This is a test
	
# This is a section
	
This is a test`

	testDocWithLink = `# Hello

This is a test [link](./doc.md)

This is a test section [link](./doc.md#this-is-a-section)`
)

func benchLinktransformer(b *testing.B) {
	b.Helper()

	tmpDir, err := os.MkdirTemp("", "bench-test")
	testutil.Ok(b, err)
	b.Cleanup(func() { testutil.Ok(b, os.RemoveAll(tmpDir)) })

	testutil.Ok(b, os.MkdirAll(filepath.Join(tmpDir, "repo", "docs"), os.ModePerm))
	testutil.Ok(b, os.WriteFile(filepath.Join(tmpDir, "repo", "docs", "doc.md"), []byte(testDoc), os.ModePerm))
	testutil.Ok(b, os.WriteFile(filepath.Join(tmpDir, "repo", "docs", "links.md"), []byte(testDocWithLink), os.ModePerm))
	anchorDir := filepath.Join(tmpDir, "repo", "docs")
	logger := log.NewLogfmtLogger(os.Stderr)

	file, err := os.OpenFile(filepath.Join(tmpDir, "repo", "docs", "links.md"), os.O_RDONLY, 0)
	testutil.Ok(b, err)
	defer file.Close()

	f := mdformatter.New(context.Background(), mdformatter.WithLinkTransformer(MustNewValidator(logger, []byte(""), anchorDir, nil)))
	b.ResetTimer()
	b.Run("Validator", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			testutil.Ok(b, f.Format(file, &testBuf))
		}
	})
}

func Benchmark_Linktransformer(b *testing.B) { benchLinktransformer(b) }
