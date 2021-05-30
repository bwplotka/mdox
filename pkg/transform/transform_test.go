// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package transform

import (
	"fmt"
	"testing"

	"github.com/efficientgo/tools/pkg/testutil"
	"github.com/pkg/errors"
)

func TestNewTargetRelPath(t *testing.T) {
	for _, tcase := range []struct {
		glob    string
		relPath string
		path    string

		expected    string
		expectedErr error
	}{
		{
			relPath: "testdata/file.md", glob: "testdata/file.md",
			path:     "",
			expected: "testdata/file.md",
		},
		{
			relPath: "testdata/file.md", glob: "testdata/file.md",
			path:     "yolo.ext",
			expected: "testdata/yolo.ext",
		},
		{
			relPath: "testdata/file.md", glob: "testdata/file.md",
			path:     "/yolo.ext",
			expected: "yolo.ext",
		},
		{
			relPath: "testdata/file.md", glob: "testdata/file.md",
			path:     "/*",
			expected: "file.md",
		},
		{
			relPath: "testdata/file.md", glob: "testdata/file.md",
			path:     "/boom/*",
			expected: "boom/file.md",
		},
		{
			relPath: "testdata/file.md", glob: "testdata/file.md",
			path:     "boom/*",
			expected: "testdata/boom/file.md",
		},
		{
			relPath: "testdata/file.md", glob: "testdata/file.md",
			path:     "/boom/**",
			expected: "boom/testdata/file.md",
		},
		{
			relPath: "testdata/file.md", glob: "testdata/file.md",
			path:        "boom/**",
			expectedErr: errors.New("path has to be absolute if suffix /** is used, got boom/**"),
		},
		{
			relPath: "testdata/file.md", glob: "testdata/file.md",
			path:     "/../1static/**",
			expected: "../1static/testdata/file.md",
		},
		{
			relPath: "../teststatic/logo4.png", glob: "../teststatic/**",
			path:     "/favicons/**",
			expected: "favicons/logo4.png",
		},
		{
			relPath: "testdata/file.md", glob: "testdata/**",
			path:     "yolo/*",
			expected: "yolo/file.md",
		},
		{
			relPath: "testdata/diff/file.md", glob: "testdata/**",
			path:     "yolo/*",
			expected: "diff/yolo/file.md",
		},
	} {
		t.Run(fmt.Sprintf("%+v", tcase), func(t *testing.T) {
			o, err := TransformationConfig{Glob: tcase.glob, Path: tcase.path}.targetRelPath(tcase.relPath)
			if tcase.expectedErr != nil {
				testutil.NotOk(t, err)
				testutil.Equals(t, tcase.expectedErr.Error(), err.Error())
				return
			}
			testutil.Ok(t, err)
			testutil.Equals(t, tcase.expected, o)
		})
	}

}
