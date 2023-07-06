// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package mdformatter

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/efficientgo/core/testutil"
)

var testBuf bytes.Buffer

func benchMdformatter(filename string, b *testing.B) {
	file, err := os.OpenFile(filename, os.O_RDONLY, 0)
	testutil.Ok(b, err)
	defer file.Close()

	f := New(context.Background())
	b.ResetTimer()
	b.Run(filename, func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			testutil.Ok(b, f.Format(file, &testBuf))
		}
	})
}

func Benchmark_Mdformatter(b *testing.B) { benchMdformatter("testdata/not_formatted.md", b) }
