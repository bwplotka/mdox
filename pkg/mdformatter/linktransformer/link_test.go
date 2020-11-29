// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package linktransformer

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/bwplotka/mdox/pkg/mdformatter"
	"github.com/bwplotka/mdox/pkg/testutil"
	"github.com/go-kit/kit/log"
)

const (
	testDocWithLinks = `[1](http://myproject.example.com/not-docs.md) [2](.)

# Yolo

[3](#yolo)

# Yolo 2

[4](http://myproject.example.com/tip/doc2.md) [5](http://myproject.example.com/v0.15.0/doc2.md) [6](http://not.myproject.example.com/tip/doc2.md)

[7](http://not.myproject.example.com/tip/a/doc.md#yolo) [8](http://myproject.example.com/tip/a/doc.md) [9](http://not.myproject.example.com/tip/doc2.md#yolo-2)

[10](http://myproject.example.com/tip/a/does_not_exists_file.md) [11](https://myproject.example.com/tip/a/does_not_exists_file2) [12](http://myproject.example.com/tip/does_not_exists/does_not_exists_dir.md)

[11](/doc2.md) [12](/a/doc.md#yolo) [13](../doc2.md) [14](../a/../a/../a/../a/doc.md) [15](doc.md)
`
)

func TestLocalizer_TransformDestination(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "test-localizer")
	testutil.Ok(t, err)
	t.Cleanup(func() { testutil.Ok(t, os.RemoveAll(tmpDir)) })

	testutil.Ok(t, os.MkdirAll(filepath.Join(tmpDir, "repo", "docs", "a"), os.ModePerm))
	testutil.Ok(t, ioutil.WriteFile(filepath.Join(tmpDir, "repo", "docs", "a", "doc.md"), []byte(testDocWithLinks), os.ModePerm))
	testutil.Ok(t, ioutil.WriteFile(filepath.Join(tmpDir, "repo", "docs", "doc2.md"), []byte(testDocWithLinks), os.ModePerm))

	logger := log.NewLogfmtLogger(os.Stderr)
	anchorDir := filepath.Join(tmpDir, "repo", "docs")
	t.Run("no link check, just formatting check should pass.", func(t *testing.T) {
		diff, err := mdformatter.IsFormatted(context.TODO(), logger, []string{
			filepath.Join(tmpDir, "repo", "docs", "a", "doc.md"),
			filepath.Join(tmpDir, "repo", "docs", "doc2.md"),
		})
		testutil.Ok(t, err)
		testutil.Equals(t, 0, len(diff), diff.String())
	})

	t.Run("no domain specified", func(t *testing.T) {
		diff, err := mdformatter.IsFormatted(context.TODO(), logger, []string{filepath.Join(tmpDir, "repo", "docs", "a", "doc.md")}, mdformatter.WithLinkTransformer(
			NewLocalizer(logger, regexp.MustCompile(`^$`), anchorDir),
		))
		testutil.Ok(t, err)
		testutil.Equals(t, 1, len(diff), diff.String())
		testutil.Equals(t, fmt.Sprintf(`--- %s/repo/docs/a/doc.md
+++ %s/repo/docs/a/doc.md (formatted)
@@ -11,1 +11,1 @@

 [10](http://myproject.example.com/tip/a/does_not_exists_file.md) [11](https://myproject.example.com/tip/a/does_not_exists_file2) [12](http://myproject.example.com/tip/does_not_exists/does_not_exists_dir.md)

-[11](/doc2.md) [12](/a/doc.md#yolo) [13](../doc2.md) [14](../a/../a/../a/../a/doc.md) [15](doc.md)
+[11](../doc2.md) [12](#yolo) [13](../doc2.md) [14](.) [15](.)

`, tmpDir, tmpDir), diff.String())
	})

	t.Run("domain specified, but without full path", func(t *testing.T) {
		diff, err := mdformatter.IsFormatted(context.TODO(), logger, []string{filepath.Join(tmpDir, "repo", "docs", "a", "doc.md")}, mdformatter.WithLinkTransformer(
			NewLocalizer(logger, regexp.MustCompile(`myproject.example.com`), anchorDir),
		))
		testutil.Ok(t, err)
		testutil.Equals(t, 1, len(diff), diff.String())
		testutil.Equals(t, fmt.Sprintf(`--- %s/repo/docs/a/doc.md
+++ %s/repo/docs/a/doc.md (formatted)
@@ -11,1 +11,1 @@

 [10](http://myproject.example.com/tip/a/does_not_exists_file.md) [11](https://myproject.example.com/tip/a/does_not_exists_file2) [12](http://myproject.example.com/tip/does_not_exists/does_not_exists_dir.md)

-[11](/doc2.md) [12](/a/doc.md#yolo) [13](../doc2.md) [14](../a/../a/../a/../a/doc.md) [15](doc.md)
+[11](../doc2.md) [12](#yolo) [13](../doc2.md) [14](.) [15](.)

`, tmpDir, tmpDir), diff.String())
	})

	t.Run("domain specified", func(t *testing.T) {
		diff, err := mdformatter.IsFormatted(context.TODO(), logger, []string{filepath.Join(tmpDir, "repo", "docs", "a", "doc.md")}, mdformatter.WithLinkTransformer(
			NewLocalizer(logger, regexp.MustCompile(`myproject.example.com/tip/`), anchorDir),
		))
		testutil.Ok(t, err)
		testutil.Equals(t, 1, len(diff), diff.String())
		testutil.Equals(t, fmt.Sprintf(`--- %s/repo/docs/a/doc.md
+++ %s/repo/docs/a/doc.md (formatted)
@@ -5,3 +5,3 @@

 # Yolo 2

-[4](http://myproject.example.com/tip/doc2.md) [5](http://myproject.example.com/v0.15.0/doc2.md) [6](http://not.myproject.example.com/tip/doc2.md)
+[4](../doc2.md) [5](http://myproject.example.com/v0.15.0/doc2.md) [6](../doc2.md)

-[7](http://not.myproject.example.com/tip/a/doc.md#yolo) [8](http://myproject.example.com/tip/a/doc.md) [9](http://not.myproject.example.com/tip/doc2.md#yolo-2)
+[7](#yolo) [8](.) [9](../doc2.md#yolo-2)

 [10](http://myproject.example.com/tip/a/does_not_exists_file.md) [11](https://myproject.example.com/tip/a/does_not_exists_file2) [12](http://myproject.example.com/tip/does_not_exists/does_not_exists_dir.md)

-[11](/doc2.md) [12](/a/doc.md#yolo) [13](../doc2.md) [14](../a/../a/../a/../a/doc.md) [15](doc.md)
+[11](../doc2.md) [12](#yolo) [13](../doc2.md) [14](.) [15](.)

`, tmpDir, tmpDir), diff.String())
	})
}

func TestValidator_TransformDestination(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "test-validator")
	testutil.Ok(t, err)
	t.Cleanup(func() { testutil.Ok(t, os.RemoveAll(tmpDir)) })

	testutil.Ok(t, os.MkdirAll(filepath.Join(tmpDir, "repo", "docs", "a"), os.ModePerm))
	testutil.Ok(t, os.MkdirAll(filepath.Join(tmpDir, "repo", "docs", "test"), os.ModePerm))
	testutil.Ok(t, ioutil.WriteFile(filepath.Join(tmpDir, "repo", "docs", "a", "doc.md"), []byte(testDocWithLinks), os.ModePerm))
	testutil.Ok(t, ioutil.WriteFile(filepath.Join(tmpDir, "repo", "docs", "doc2.md"), []byte(testDocWithLinks), os.ModePerm))

	logger := log.NewLogfmtLogger(os.Stderr)
	anchorDir := filepath.Join(tmpDir, "repo", "docs")
	t.Run("check valid link", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "repo", "docs", "test", "valid-link.md")
		testutil.Ok(t, ioutil.WriteFile(testFile, []byte("https://bwplotka.dev/about\n"), os.ModePerm))

		diff, err := mdformatter.IsFormatted(context.TODO(), logger, []string{testFile})
		testutil.Ok(t, err)
		testutil.Equals(t, 0, len(diff), diff.String())

		diff, err = mdformatter.IsFormatted(context.TODO(), logger, []string{testFile}, mdformatter.WithLinkTransformer(
			NewValidator(logger, regexp.MustCompile(`^$`), anchorDir),
		))
		testutil.Ok(t, err)
		testutil.Equals(t, 0, len(diff), diff.String())
	})

	t.Run("check valid local links", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "repo", "docs", "test", "valid-local-links.md")
		testutil.Ok(t, ioutil.WriteFile(testFile, []byte(`# yolo

[1](.) [2](#yolo) [3](../test/valid-local-links.md) [4](../test/valid-local-links.md#yolo) [5](../a/doc.md)
`), os.ModePerm))

		diff, err := mdformatter.IsFormatted(context.TODO(), logger, []string{testFile})
		testutil.Ok(t, err)
		testutil.Equals(t, 0, len(diff), diff.String())

		diff, err = mdformatter.IsFormatted(context.TODO(), logger, []string{testFile}, mdformatter.WithLinkTransformer(
			NewValidator(logger, regexp.MustCompile(`^$`), anchorDir),
		))
		testutil.Ok(t, err)
		testutil.Equals(t, 0, len(diff), diff.String())
	})

	t.Run("check invalid local links", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "repo", "docs", "test", "invalid-local-links.md")
		testutil.Ok(t, ioutil.WriteFile(testFile, []byte(`# yolo

[1](.) [2](#not-yolo) [3](../test2/invalid-local-links.md) [4](../test/invalid-local-links.md#not-yolo) [5](../test/doc.md)
`), os.ModePerm))

		diff, err := mdformatter.IsFormatted(context.TODO(), logger, []string{testFile})
		testutil.Ok(t, err)
		testutil.Equals(t, 0, len(diff), diff.String())

		_, err = mdformatter.IsFormatted(context.TODO(), logger, []string{testFile}, mdformatter.WithLinkTransformer(
			NewValidator(logger, regexp.MustCompile(`^$`), anchorDir),
		))
		testutil.NotOk(t, err)
		testutil.Equals(t, "found 4 problems with links.", err.Error())
	})

	t.Run("check 404 link", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "repo", "docs", "test", "invalid-link.md")
		testutil.Ok(t, ioutil.WriteFile(testFile, []byte("https://bwplotka.dev/does-not-exists\n"), os.ModePerm))

		diff, err := mdformatter.IsFormatted(context.TODO(), logger, []string{testFile})
		testutil.Ok(t, err)
		testutil.Equals(t, 0, len(diff), diff.String())

		_, err = mdformatter.IsFormatted(context.TODO(), logger, []string{testFile}, mdformatter.WithLinkTransformer(
			NewValidator(logger, regexp.MustCompile(`^$`), anchorDir),
		))
		testutil.NotOk(t, err)
		testutil.Equals(t, "found 1 problems with links.", err.Error())
	})

	t.Run("check 404 link, ignored", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "repo", "docs", "test", "invalid-link2.md")
		testutil.Ok(t, ioutil.WriteFile(testFile, []byte("https://bwplotka.dev/does-not-exists\n"), os.ModePerm))

		diff, err := mdformatter.IsFormatted(context.TODO(), logger, []string{testFile})
		testutil.Ok(t, err)
		testutil.Equals(t, 0, len(diff), diff.String())

		_, err = mdformatter.IsFormatted(context.TODO(), logger, []string{testFile}, mdformatter.WithLinkTransformer(
			NewValidator(logger, regexp.MustCompile(`://bwplotka.dev`), anchorDir),
		))
		testutil.Ok(t, err)
	})
}

func TestGetAnchorDir(t *testing.T) {
	pwd, err := os.Getwd()
	testutil.Ok(t, err)

	// Consider parametrizing this.
	anchorDir, err := GetAnchorDir("", []string{})
	testutil.Ok(t, err)
	testutil.Equals(t, pwd, anchorDir)

	anchorDir, err = GetAnchorDir("/root", []string{})
	testutil.Ok(t, err)
	testutil.Equals(t, "/root", anchorDir)

	_, err = GetAnchorDir("/root", []string{"/root/something.md", "/home/something/file.md", "/root/a/b/c/file.md"})
	testutil.NotOk(t, err)
	testutil.Equals(t, "anchorDir \"/root\" is not in path of provided file \"/home/something/file.md\"", err.Error())

	_, err = GetAnchorDir("/root", []string{"/root/something.md", "/root/something/file.md", "/root/a/b/c/file.md"})
	testutil.Ok(t, err)
	testutil.Equals(t, "/root", anchorDir)
}
