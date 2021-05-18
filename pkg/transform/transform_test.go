// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package transform_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/bwplotka/mdox/pkg/transform"
	"github.com/efficientgo/tools/pkg/testutil"
	"github.com/go-kit/kit/log"
)

func TestTransform(t *testing.T) {
	const (
		tmpDir   = "testdata/tmp"
		testData = "testdata"
	)
	//defer func() { _ = os.RemoveAll(tmpDir) }()

	logger := log.NewLogfmtLogger(os.Stdout)

	testutil.Ok(t, os.RemoveAll(tmpDir))
	t.Run("mdox1.yaml", func(t *testing.T) {
		testutil.Ok(t, transform.Dir(context.Background(), logger, filepath.Join(testData, "mdox1.yaml")))

		// TODO(bwplotka): Assert on output dir (asserted manually so far).
	})
	t.Run("mdox2.yaml", func(t *testing.T) {
		testutil.Ok(t, transform.Dir(context.Background(), logger, filepath.Join(testData, "mdox2.yaml")))

		// TODO(bwplotka): Assert on output dir (asserted manually so far).
	})
}
