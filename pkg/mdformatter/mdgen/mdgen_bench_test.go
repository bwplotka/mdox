// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package mdgen

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/bwplotka/mdox/pkg/mdformatter"
	"github.com/efficientgo/core/testutil"
)

var testBuf bytes.Buffer

func benchMdgen(filename string, b *testing.B) {
	file, err := os.OpenFile(filename, os.O_RDONLY, 0)
	testutil.Ok(b, err)
	defer file.Close()

	f := mdformatter.New(context.Background(), mdformatter.WithCodeBlockTransformer(NewCodeBlockTransformer()))
	b.ResetTimer()

	b.Run(filename, func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			testutil.Ok(b, f.Format(file, &testBuf))
		}
	})
}

func Benchmark_Mdgen_Sleep2(b *testing.B) { benchMdgen("benchdata/sleep2.md", b) }

func Benchmark_Mdgen_Sleep5(b *testing.B) { benchMdgen("benchdata/sleep5.md", b) }

func Benchmark_Mdgen_GoHelp(b *testing.B) { benchMdgen("benchdata/gohelp.md", b) }
