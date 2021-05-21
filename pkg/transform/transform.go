// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package transform

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/bwplotka/mdox/pkg/mdformatter"
	"github.com/efficientgo/tools/pkg/errcapture"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

// Dir transforms directory using given configuration file.
func Dir(ctx context.Context, logger log.Logger, configFile string) error {
	c, err := parseConfigFile(configFile)
	if err != nil {
		return err
	}

	for _, d := range []string{c.OutputDir, c.OutputStaticDir} {
		if d == "" {
			continue
		}

		_, err = os.Stat(d)
		if err != nil && !os.IsNotExist(err) {
			return err
		}

		if !os.IsNotExist(err) {
			if err := os.RemoveAll(d); err != nil {
				return err
			}
		}
		if err := os.MkdirAll(d, os.ModePerm); err != nil {
			return err
		}

		if c.GitIgnored {
			if err = ioutil.WriteFile(filepath.Join(d, ".gitignore"), []byte("*"), os.ModePerm); err != nil {
				return err
			}
		}

	}

	tr := &transformer{
		ctx:    ctx,
		c:      c,
		logger: logger,

		linkTransformer: &relLinkTransformer{
			localLinksStyle: c.LocalLinksStyle,
			inputDir:        c.InputDir,
			outputDir:       c.OutputDir,
			oldRelPath:      map[string]string{},
			newAbsRelPath:   map[string]string{},
		},
	}

	for _, extra := range c.ExtraInputGlobs {
		matches, err := filepath.Glob(extra)
		if err != nil {
			return err
		}

		for _, m := range matches {
			info, err := os.Stat(m)
			if err := tr.transformFile(m, info, err); err != nil {
				return err
			}
		}
	}

	// Move markdown files, preserving dir structure to output while preprocessing files.
	if err := filepath.Walk(c.InputDir, tr.transformFile); err != nil {
		return errors.Wrap(err, "walk")
	}

	// Once we did all the changes, change links.
	return mdformatter.Format(ctx, logger, tr.filesToLinkAdjust, mdformatter.WithLinkTransformer(tr.linkTransformer))
}

type transformer struct {
	ctx    context.Context
	c      Config
	logger log.Logger

	filesToLinkAdjust []string
	linkTransformer   *relLinkTransformer
}

func (t *transformer) transformFile(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}
	if info.IsDir() || path == t.c.InputDir {
		return nil
	}

	// All relative paths are in relation to either input or output dirs.
	relPath, err := filepath.Rel(t.c.InputDir, path)
	if err != nil {
		return errors.Wrap(err, "rel path to input dir")
	}

	if filepath.Ext(path) != ".md" {
		out := t.c.OutputStaticDir
		if out == "" {
			out = t.c.OutputDir
		}
		level.Debug(t.logger).Log("msg", "copying static file", "in", path, "target", filepath.Join(out, relPath))
		return copyFiles(path, filepath.Join(out, relPath))
	}

	// Copy while preserving structure and tolerating custom mapping.
	target := filepath.Join(t.c.OutputDir, relPath)

	tr, ok := firstMatch(relPath, t.c.Transformations)
	if !ok {
		t.filesToLinkAdjust = append(t.filesToLinkAdjust, target)
		level.Debug(t.logger).Log("msg", "copying without transformation", "in", path, "absRelPath", relPath, "target", target)
		return copyFiles(path, target)
	}

	defer func() {
		t.filesToLinkAdjust = append(t.filesToLinkAdjust, target)
	}()

	var opts []mdformatter.Option
	newAbsRelPath := newTargetAbsRelPath(relPath, tr)
	if newAbsRelPath != "/"+relPath {
		t.linkTransformer.oldRelPath[newAbsRelPath] = relPath
		t.linkTransformer.newAbsRelPath[relPath] = newAbsRelPath
	}

	target = filepath.Join(t.c.OutputDir, newAbsRelPath)
	level.Debug(t.logger).Log("msg", "copying with transformation", "in", path, "absRelPath", relPath, "target", target)
	if err := copyFiles(path, target); err != nil {
		return err
	}

	if tr.FrontMatter != nil {
		firstHeader, rest, err := popFirstHeader(path)
		if err != nil {
			return errors.Wrap(err, "read first header")
		}

		if err := ioutil.WriteFile(target, rest, os.ModePerm); err != nil {
			return err
		}
		_, originFilename := filepath.Split(path)
		_, targetFilename := filepath.Split(target)
		opts = append(opts, mdformatter.WithFrontMatterTransformer(&frontMatterTransformer{
			localLinksStyle: t.c.LocalLinksStyle,
			c:               tr.FrontMatter,
			origin: FrontMatterOrigin{
				Filename:    originFilename,
				FirstHeader: firstHeader,
				LastMod:     info.ModTime().String(),
			},
			target: FrontMatterTarget{
				FileName: targetFilename,
			},
		}))
	}
	return mdformatter.Format(t.ctx, t.logger, []string{target}, opts...)
}

type relLinkTransformer struct {
	localLinksStyle LinksStyle

	inputDir      string
	outputDir     string
	oldRelPath    map[string]string
	newAbsRelPath map[string]string
}

func (r *relLinkTransformer) TransformDestination(ctx mdformatter.SourceContext, destination []byte) ([]byte, error) {
	split := strings.Split(string(destination), "#")
	dest := split[0]

	if strings.Contains(dest, "://") || filepath.IsAbs(dest) || strings.HasPrefix(string(destination), "#") {
		return destination, nil
	}

	// absRelPath is an absolute path, but relatively to output dir (has `/` upfront).
	currentAbsRelPath := strings.TrimPrefix(ctx.Filepath, r.outputDir)
	if filepath.Join(currentAbsRelPath, dest) == ctx.Filepath {
		// Pointing to self.
		_, file := filepath.Split(ctx.Filepath)
		if len(split) > 1 {
			return []byte(file + "#" + split[1]), nil
		}
		return []byte(file), nil
	}

	currentAbsRelDir := filepath.Dir(currentAbsRelPath)

	// Do we changed. Try to navigate from current, transformed path into what was before.
	oldRelDir := currentAbsRelDir
	if old, ok := r.oldRelPath[currentAbsRelPath]; ok {
		// Remove absoluteness for proper relation check.
		currRelPath := strings.TrimPrefix(currentAbsRelPath, "/")
		change, err := filepath.Rel(filepath.Dir(old), filepath.Dir(currRelPath))
		if err != nil {
			// Possible error: old path is before currRelPath. This can happen if the input file was grabbed from outside.
			// Try otherwise.
			change, err = filepath.Rel(filepath.Dir(currRelPath), filepath.Join(filepath.Dir(old)))
			if err != nil {
				return nil, err
			}
			oldRelDir = filepath.Join(change, currentAbsRelDir)
		} else {
			oldRelDir = filepath.Join(currentAbsRelDir, change)
		}
	}
	oldRelDestination := filepath.Join(oldRelDir, dest)

	// Evaluate this path to make sure we cover case when we go outside of our "absolute" root which is output dir.
	absOldDest, err := filepath.Abs(filepath.Join(r.inputDir, oldRelDestination))
	if err != nil {
		return nil, err
	}

	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	oldRelDestination, err = filepath.Rel(filepath.Join(wd, r.inputDir), absOldDest)
	if err != nil {
		return nil, err
	}

	oldAbsDestination := "/" + oldRelDestination
	// Does the link points to something that changed?
	if absNewRelPath, ok := r.newAbsRelPath[oldRelDestination]; ok {
		oldAbsDestination = absNewRelPath
	}

	newDest, err := filepath.Rel(currentAbsRelDir, oldAbsDestination)
	if err != nil {
		return nil, errors.Wrap(err, "relation between new and old file path")
	}

	if newDest == "." {
		newDest = ""
	} else if r.localLinksStyle == Hugo && filepath.Ext(dest) == ".md" {
		// Because all links are normally files, in Hugo those are literally URL paths (kind of "dirs").
		// This is why we need to add ../ to them.
		newDest = filepath.Join("..", newDest) + "/"

		// All slugs and paths are converted to lower case on hugo too, so do this too links.
		newDest = strings.ToLower(newDest)
	}

	if len(split) > 1 {
		return []byte(newDest + "#" + split[1]), nil
	}
	return []byte(newDest), nil
}

func (r *relLinkTransformer) Close(mdformatter.SourceContext) error { return nil }

type frontMatterTransformer struct {
	localLinksStyle LinksStyle
	c               *FrontMatterConfig

	// Vars.
	origin FrontMatterOrigin
	target FrontMatterTarget
}

type FrontMatterOrigin struct {
	Filename    string
	FirstHeader string
	LastMod     string
}

type FrontMatterTarget struct {
	FileName string
}

func (f *frontMatterTransformer) TransformFrontMatter(ctx mdformatter.SourceContext, frontMatter map[string]interface{}) ([]byte, error) {
	b := bytes.Buffer{}
	if err := f.c._template.Execute(&b, struct {
		Origin      FrontMatterOrigin
		Target      FrontMatterTarget
		FrontMatter map[string]interface{}
	}{
		Origin:      f.origin,
		Target:      f.target,
		FrontMatter: frontMatter,
	}); err != nil {
		return nil, err
	}

	m := map[string]interface{}{}
	if err := yaml.Unmarshal(b.Bytes(), m); err != nil {
		return nil, errors.Wrapf(err, "generated template for %v is not a valid yaml", ctx.Filepath)
	}

	if f.localLinksStyle == Hugo {
		if _, ok := m["slug"]; !ok {
			m["slug"] = f.target.FileName
		}
	}

	return mdformatter.FormatFrontMatter(m)
}

func (f *frontMatterTransformer) Close(mdformatter.SourceContext) error { return nil }

func firstMatch(absRelPath string, trs []*TransformationConfig) (*TransformationConfig, bool) {
	for _, tr := range trs {
		if tr._glob.Match(absRelPath) {
			return tr, true
		}
	}
	return nil, false
}

func newTargetAbsRelPath(relPath string, tr *TransformationConfig) string {
	if tr.Path == "" {
		return "/" + relPath
	}

	if filepath.IsAbs(tr.Path) {
		return tr.Path
	}

	return "/" + filepath.Join(filepath.Dir(relPath), tr.Path)
}

func copyFiles(src, dst string) (err error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return errors.Wrap(err, "cpy source")
	}

	if !sourceFileStat.Mode().IsRegular() {
		return errors.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return errors.Wrap(err, "cpy source")
	}
	defer errcapture.Close(&err, source, "src close")

	if err := os.MkdirAll(filepath.Dir(dst), os.ModePerm); err != nil {
		return err
	}

	destination, err := os.Create(dst)
	if err != nil {
		return errors.Wrap(err, "cpy dest")
	}
	defer errcapture.Close(&err, destination, "dst close")

	_, err = io.Copy(destination, source)
	return err
}

// TODO(bwplotka): Use formatter, remove the title etc.
// Super hacky for now.
func popFirstHeader(path string) (_ string, rest []byte, err error) {
	file, err := os.Open(path)
	if err != nil {
		return "", nil, err
	}
	defer errcapture.Close(&err, file, "close file")

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		text := scanner.Text()
		if strings.HasPrefix(text, "#") {
			if _, err := file.Seek(int64(len(text)), 0); err != nil {
				return "", nil, errors.Wrap(err, "seek")
			}
			rest, err := ioutil.ReadAll(file)
			if err != nil {
				return "", nil, errors.Wrap(err, "read")
			}

			return strings.TrimPrefix(text, "# "), rest, scanner.Err()
		}
	}
	if err := scanner.Err(); err != nil {
		return "", nil, err
	}
	return "", nil, errors.New("No header found")
}
