// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package main

import (
	"os"
	"testing"

	"github.com/efficientgo/core/testutil"
)

func TestValidateAnchorDir(t *testing.T) {
	pwd, err := os.Getwd()
	testutil.Ok(t, err)

	// Consider parametrizing this.
	anchorDir, err := validateAnchorDir("", []string{})
	testutil.Ok(t, err)
	testutil.Equals(t, pwd, anchorDir)

	anchorDir, err = validateAnchorDir("/home", []string{})
	testutil.Ok(t, err)
	testutil.Equals(t, "/home", anchorDir)

	anchorDir, err = validateAnchorDir(".", []string{})
	testutil.Ok(t, err)
	testutil.Equals(t, pwd, anchorDir)

	_, err = validateAnchorDir("/root", []string{"/root/something.md", "/home/something/file.md", "/root/a/b/c/file.md"})
	testutil.NotOk(t, err)
	testutil.Equals(t, "anchorDir \"/root\" is not in path of provided file \"/home/something/file.md\"", err.Error())

	anchorDir, err = validateAnchorDir("/root", []string{"/root/something.md", "/root/something/file.md", "/root/a/b/c/file.md"})
	testutil.Ok(t, err)
	testutil.Equals(t, "/root", anchorDir)
}
