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
	"github.com/efficientgo/tools/core/pkg/testutil"
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

[11](/doc2.md) [12](/a/doc.md#yolo) [13](../doc2.md) [14](../a/../a/../a/../a/doc.md) [15](doc.md) [16](doc2.md/#yolo-2)
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

-[11](/doc2.md) [12](/a/doc.md#yolo) [13](../doc2.md) [14](../a/../a/../a/../a/doc.md) [15](doc.md) [16](doc2.md/#yolo-2)
+[11](../doc2.md) [12](#yolo) [13](../doc2.md) [14](.) [15](.) [16](../doc2.md#yolo-2)

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

-[11](/doc2.md) [12](/a/doc.md#yolo) [13](../doc2.md) [14](../a/../a/../a/../a/doc.md) [15](doc.md) [16](doc2.md/#yolo-2)
+[11](../doc2.md) [12](#yolo) [13](../doc2.md) [14](.) [15](.) [16](../doc2.md#yolo-2)

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

-[11](/doc2.md) [12](/a/doc.md#yolo) [13](../doc2.md) [14](../a/../a/../a/../a/doc.md) [15](doc.md) [16](doc2.md/#yolo-2)
+[11](../doc2.md) [12](#yolo) [13](../doc2.md) [14](.) [15](.) [16](../doc2.md#yolo-2)

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
			MustNewValidator(logger, []byte(""), anchorDir),
		))
		testutil.Ok(t, err)
		testutil.Equals(t, 0, len(diff), diff.String())
	})

	t.Run("check valid but same link in diff files", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "repo", "docs", "test", "valid-link.md")
		testFile2 := filepath.Join(tmpDir, "repo", "docs", "test", "valid-link2.md")
		testutil.Ok(t, ioutil.WriteFile(testFile, []byte("https://bwplotka.dev/about\n"), os.ModePerm))
		testutil.Ok(t, ioutil.WriteFile(testFile2, []byte("https://bwplotka.dev/about\n"), os.ModePerm))

		diff, err := mdformatter.IsFormatted(context.TODO(), logger, []string{testFile, testFile2})
		testutil.Ok(t, err)
		testutil.Equals(t, 0, len(diff), diff.String())

		diff, err = mdformatter.IsFormatted(context.TODO(), logger, []string{testFile, testFile2}, mdformatter.WithLinkTransformer(
			MustNewValidator(logger, []byte(""), anchorDir),
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
			MustNewValidator(logger, []byte(""), anchorDir),
		))
		testutil.Ok(t, err)
		testutil.Equals(t, 0, len(diff), diff.String())
	})

	t.Run("check valid local links with dash", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "repo", "docs", "test", "valid-local-links-with-dash.md")
		testutil.Ok(t, ioutil.WriteFile(testFile, []byte(`# Expose UI on a sub-path

[1](#expose-ui-on-a-sub-path)

# Run-time deduplication of HA groups

[2](#run-time-deduplication-of-ha-groups)
`), os.ModePerm))

		diff, err := mdformatter.IsFormatted(context.TODO(), logger, []string{testFile})
		testutil.Ok(t, err)
		testutil.Equals(t, 0, len(diff), diff.String())

		diff, err = mdformatter.IsFormatted(context.TODO(), logger, []string{testFile}, mdformatter.WithLinkTransformer(
			MustNewValidator(logger, []byte(""), anchorDir),
		))
		testutil.Ok(t, err)
		testutil.Equals(t, 0, len(diff), diff.String())
	})

	t.Run("check valid local links in diff language", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "repo", "docs", "test", "valid-local-links-diff-lang.md")
		testutil.Ok(t, ioutil.WriteFile(testFile, []byte(`# Twój wkład w dokumentację

[1](#twój-wkład-w-dokumentację)

## Hugo का उपयोग करते हुए स्थानीय रूप से साइट चलाना

[2](#hugo-का-उपयोग-करते-हुए-स्थानीय-रूप-से-साइट-चलाना)
`), os.ModePerm))

		diff, err := mdformatter.IsFormatted(context.TODO(), logger, []string{testFile})
		testutil.Ok(t, err)
		testutil.Equals(t, 0, len(diff), diff.String())

		diff, err = mdformatter.IsFormatted(context.TODO(), logger, []string{testFile}, mdformatter.WithLinkTransformer(
			MustNewValidator(logger, []byte(""), anchorDir),
		))
		testutil.Ok(t, err)
		testutil.Equals(t, 0, len(diff), diff.String())
	})

	t.Run("check invalid local links", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "repo", "docs", "test", "invalid-local-links.md")
		filePath := "/repo/docs/test/invalid-local-links.md"
		wdir, err := os.Getwd()
		testutil.Ok(t, err)
		relDirPath, err := filepath.Rel(wdir, tmpDir)
		testutil.Ok(t, err)
		testutil.Ok(t, ioutil.WriteFile(testFile, []byte(`# yolo

[1](.) [2](#not-yolo) [3](../test2/invalid-local-links.md) [4](../test/invalid-local-links.md#not-yolo) [5](../test/doc.md)
`), os.ModePerm))

		diff, err := mdformatter.IsFormatted(context.TODO(), logger, []string{testFile})
		testutil.Ok(t, err)
		testutil.Equals(t, 0, len(diff), diff.String())

		_, err = mdformatter.IsFormatted(context.TODO(), logger, []string{testFile}, mdformatter.WithLinkTransformer(
			MustNewValidator(logger, []byte(""), anchorDir),
		))
		testutil.NotOk(t, err)

		testutil.Equals(t, fmt.Sprintf("%v: 4 errors: "+
			"%v:3: link ../test2/invalid-local-links.md, normalized to: %v/repo/docs/test2/invalid-local-links.md: file not found; "+
			"%v:3: link ../test/invalid-local-links.md#not-yolo, normalized to: link %v/repo/docs/test/invalid-local-links.md#not-yolo, existing ids: [yolo]: file exists, but does not have such id; "+
			"%v:3: link ../test/doc.md, normalized to: %v/repo/docs/test/doc.md: file not found; "+
			"%v:3: link #not-yolo, normalized to: link %v/repo/docs/test/invalid-local-links.md#not-yolo, existing ids: [yolo]: file exists, but does not have such id",
			tmpDir+filePath, relDirPath+filePath, tmpDir, relDirPath+filePath, tmpDir, relDirPath+filePath, tmpDir, relDirPath+filePath, tmpDir), err.Error())
	})

	t.Run("check 404 link", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "repo", "docs", "test", "invalid-link.md")
		testutil.Ok(t, ioutil.WriteFile(testFile, []byte("https://bwplotka.dev/does-not-exists\n"), os.ModePerm))
		filePath := "/repo/docs/test/invalid-link.md"
		wdir, err := os.Getwd()
		testutil.Ok(t, err)
		relDirPath, err := filepath.Rel(wdir, tmpDir)
		testutil.Ok(t, err)

		diff, err := mdformatter.IsFormatted(context.TODO(), logger, []string{testFile})
		testutil.Ok(t, err)
		testutil.Equals(t, 0, len(diff), diff.String())

		_, err = mdformatter.IsFormatted(context.TODO(), logger, []string{testFile}, mdformatter.WithLinkTransformer(
			MustNewValidator(logger, []byte(""), anchorDir),
		))
		testutil.NotOk(t, err)
		testutil.Equals(t, fmt.Sprintf("%v%v: %v%v:1: \"https://bwplotka.dev/does-not-exists\" not accessible; status code 404: Not Found", tmpDir, filePath, relDirPath, filePath), err.Error())
	})

	t.Run("check valid & 404 link with validate config", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "repo", "docs", "test", "invalid-link2.md")
		testutil.Ok(t, ioutil.WriteFile(testFile, []byte("https://www.github.com/ https://bwplotka.dev/does-not-exits\n"), os.ModePerm))

		diff, err := mdformatter.IsFormatted(context.TODO(), logger, []string{testFile})
		testutil.Ok(t, err)
		testutil.Equals(t, 0, len(diff), diff.String())

		_, err = mdformatter.IsFormatted(context.TODO(), logger, []string{testFile}, mdformatter.WithLinkTransformer(
			MustNewValidator(logger, []byte("version: 1\n\nvalidators:\n  - regex: 'bwplotka'\n    type: 'ignore'\n"), anchorDir),
		))
		testutil.Ok(t, err)
	})

	t.Run("check 404 links with ignore validate config", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "repo", "docs", "test", "links.md")
		testutil.Ok(t, ioutil.WriteFile(testFile, []byte("https://fakelink1.com/ http://fakelink2.com/ https://www.fakelink3.com/\n"), os.ModePerm))

		diff, err := mdformatter.IsFormatted(context.TODO(), logger, []string{testFile})
		testutil.Ok(t, err)
		testutil.Equals(t, 0, len(diff), diff.String())

		_, err = mdformatter.IsFormatted(context.TODO(), logger, []string{testFile}, mdformatter.WithLinkTransformer(
			MustNewValidator(logger, []byte("version: 1\n\nvalidators:\n  - regex: '(^http[s]?:\\/\\/)(www\\.)?(fakelink[0-9]\\.com\\/)'\n    type: 'ignore'\n"), anchorDir),
		))
		testutil.Ok(t, err)
	})

	t.Run("check github links with validate config", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "repo", "docs", "test", "github-link.md")
		testutil.Ok(t, ioutil.WriteFile(testFile, []byte("https://github.com/bwplotka/mdox/issues/23 https://github.com/bwplotka/mdox/pull/32 https://github.com/bwplotka/mdox/pull/27#pullrequestreview-659598194\n"), os.ModePerm))
		// This is substituted in config using PathorContent flag. But need to pass it directly here.
		repoToken := os.Getenv("GITHUB_TOKEN")

		diff, err := mdformatter.IsFormatted(context.TODO(), logger, []string{testFile})
		testutil.Ok(t, err)
		testutil.Equals(t, 0, len(diff), diff.String())

		_, err = mdformatter.IsFormatted(context.TODO(), logger, []string{testFile}, mdformatter.WithLinkTransformer(
			MustNewValidator(logger, []byte("version: 1\n\nvalidators:\n  - regex: '(^http[s]?:\\/\\/)(www\\.)?(github\\.com\\/)bwplotka\\/mdox(\\/pull\\/|\\/issues\\/)'\n    token: '"+repoToken+"'\n    type: 'githubPullsIssues'\n"), anchorDir),
		))
		testutil.Ok(t, err)
	})
}
