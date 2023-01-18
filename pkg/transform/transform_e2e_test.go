// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package transform_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/bwplotka/mdox/pkg/transform"
	"github.com/efficientgo/core/testutil"
	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
)

func TestTransform(t *testing.T) {
	const (
		tmpDir   = "testdata/tmp"
		testData = "testdata"
	)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	logger := log.NewLogfmtLogger(os.Stdout)

	testutil.Ok(t, os.RemoveAll(tmpDir))
	t.Run("mdox1.yaml", func(t *testing.T) {
		mdox1, err := os.ReadFile(filepath.Join(testData, "mdox1.yaml"))
		testutil.Ok(t, err)
		testutil.Ok(t, transform.Dir(context.Background(), logger, mdox1))
		assertDirContent(t, filepath.Join(testData, "expected", "test1"), filepath.Join(tmpDir, "test1"))
	})
	t.Run("mdox2.yaml", func(t *testing.T) {
		mdox2, err := os.ReadFile(filepath.Join(testData, "mdox2.yaml"))
		testutil.Ok(t, err)
		testutil.Ok(t, transform.Dir(context.Background(), logger, mdox2))
		assertDirContent(t, filepath.Join(testData, "expected", "test2"), filepath.Join(tmpDir, "test2"))

	})
	t.Run("mdox3.yaml", func(t *testing.T) {
		mdox2, err := os.ReadFile(filepath.Join(testData, "mdox3.yaml"))
		testutil.Ok(t, err)
		testutil.Ok(t, transform.Dir(context.Background(), logger, mdox2))
		assertDirContent(t, filepath.Join(testData, "expected", "test3"), filepath.Join(tmpDir, "test3"))

	})
}

func assertDirContent(t *testing.T, expectedDir string, gotDir string) {
	// TODO(bwplotka): Check if some garbage was not generated too!
	testutil.Ok(t, filepath.Walk(expectedDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(expectedDir, path)
		if err != nil {
			return err
		}

		expectedPath := filepath.Join(gotDir, relPath)
		if info.IsDir() {
			e, err := os.Stat(expectedPath)
			if err != nil {
				return err
			}
			if !e.IsDir() {
				return errors.Errorf("%v is not a dir, but expected dir", e)
			}
			return nil
		}

		e, err := os.Stat(expectedPath)
		if err != nil {
			return err
		}
		if e.IsDir() {
			return errors.Errorf("%v is a dir, but not expected a dir, but file", e)
		}

		expectedContent, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(expectedPath)
		if err != nil {
			return err
		}
		if !bytes.Equal(expectedContent, content) {
			return errors.Errorf("expected (from %v):\n %v\n ..got (in %v):\n %v\n", path, string(expectedContent), expectedPath, string(content))
		}
		return nil
	}))
}
