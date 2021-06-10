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
	"github.com/efficientgo/tools/core/pkg/errcapture"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

func isMDFile(path string) bool {
	return filepath.Ext(path) == ".md"
}

func prepOutputDir(d string, gitIgnored bool) error {
	_, err := os.Stat(d)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if os.IsExist(err) {
		if err := os.RemoveAll(d); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(d, os.ModePerm); err != nil {
		return err
	}

	if gitIgnored {
		if err = ioutil.WriteFile(filepath.Join(d, ".gitignore"), []byte("*"), os.ModePerm); err != nil {
			return err
		}
	}
	return nil
}

// Dir transforms directory using given configuration file.
func Dir(ctx context.Context, logger log.Logger, configFile string) error {
	c, err := parseConfigFile(configFile)
	if err != nil {
		return err
	}

	if err := prepOutputDir(c.OutputDir, c.GitIgnored); err != nil {
		return err
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
			newRelPath:      map[string]string{},
		},
	}

	for _, e := range c.ExtraInputGlobs {
		extra, err := filepath.Abs(e)
		if err != nil {
			return err
		}
		matches, err := filepath.Glob(extra)
		if err != nil {
			return err
		}

		if len(matches) == 0 {
			return errors.Errorf("no matches found for extraInputGlob %v", e)
		}

		for _, m := range matches {
			if err := filepath.Walk(m, tr.transformFile); err != nil {
				return errors.Wrap(err, "walk, extra input")
			}
		}
	}

	// Move files, preserving dir structure to output while preprocessing files.
	// For markdown files, adjust links too.
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

	// Copy while preserving structure and tolerating custom mapping.
	target := filepath.Join(t.c.OutputDir, relPath)

	defer func() {
		if isMDFile(target) {
			t.filesToLinkAdjust = append(t.filesToLinkAdjust, target)
		}
	}()

	tr, ok := firstMatch(relPath, t.c.Transformations)
	if !ok {
		level.Debug(t.logger).Log("msg", "copying without transformation", "in", path, "relPath", relPath, "target", target)
		return copyFiles(path, target)
	}

	var opts []mdformatter.Option
	newRelPath, err := tr.targetRelPath(relPath)
	if err != nil {
		return err
	}

	if strings.HasPrefix(newRelPath, "..") {
		// Silly way of propagating git ignores where needed.
		if err := prepOutputDir(filepath.Join(t.c.OutputDir, filepath.Dir(newRelPath)), t.c.GitIgnored); err != nil {
			return err
		}
	}
	target = filepath.Join(t.c.OutputDir, newRelPath)

	if newRelPath != relPath && isMDFile(target) {
		t.linkTransformer.oldRelPath[newRelPath] = relPath
		t.linkTransformer.newRelPath[relPath] = newRelPath
	}

	level.Debug(t.logger).Log("msg", "copying with transformation", "in", path, "relPath", relPath, "target", target)
	if err := copyFiles(path, target); err != nil {
		return err
	}

	if tr.FrontMatter != nil {
		if !isMDFile(target) {
			return errors.Errorf("front matter option set on file that after transformation is non-markdown: %v", target)
		}

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

	if !isMDFile(target) {
		return nil
	}

	return mdformatter.Format(t.ctx, t.logger, []string{target}, opts...)
}

type relLinkTransformer struct {
	localLinksStyle LocalLinksStyle

	inputDir   string
	outputDir  string
	oldRelPath map[string]string
	newRelPath map[string]string
}

func (r *relLinkTransformer) TransformDestination(ctx mdformatter.SourceContext, destination []byte) ([]byte, error) {
	split := strings.Split(string(destination), "#")
	relDest := split[0]
	if strings.Contains(relDest, "://") || filepath.IsAbs(relDest) || strings.HasPrefix(string(destination), "#") {
		return destination, nil
	}

	currRelPath, err := filepath.Rel(r.outputDir, ctx.Filepath)
	if err != nil {
		return nil, errors.Wrap(err, "link: rel filepath to output")
	}

	if filepath.Join(currRelPath, relDest) == ctx.Filepath {
		// Pointing to self.
		_, file := filepath.Split(ctx.Filepath)
		if len(split) > 1 {
			return []byte(file + "#" + split[1]), nil
		}
		return []byte(file), nil
	}

	// Check the situation of input file from the before conversion, what was the link targeting before conversion?
	curRelDir := filepath.Dir(currRelPath)
	oldRelDest := filepath.Join(curRelDir, relDest)
	if oldRelPath, ok := r.oldRelPath[currRelPath]; ok {
		oldRelDest, err = filepath.Rel(r.inputDir, filepath.Join(r.inputDir, filepath.Dir(oldRelPath), relDest))
		if err != nil {
			return nil, errors.Wrap(err, "link: clean old dest path")
		}
	}

	currDest := oldRelDest
	if newRelPath, ok := r.newRelPath[oldRelDest]; ok {
		currDest = newRelPath
	}

	newDest, err := filepath.Rel(filepath.Join(r.outputDir, curRelDir), filepath.Join(r.outputDir, currDest))
	if err != nil {
		return nil, errors.Wrap(err, "link: rel new dest dir with curr file dir")
	}

	if newDest == "." {
		newDest = ""
	} else if h := r.localLinksStyle.Hugo; h != nil {
		if !strings.HasSuffix(ctx.Filepath, h.IndexFileName) {
			// All links are normally just files, in Hugo those are literally URL paths (kind of "dirs").
			// This is why we need to add ../ to them.
			newDest = filepath.Join("..", newDest)
		}

		if isMDFile(newDest) {
			// All slugs and paths are converted to lower case on hugo.
			newDest = strings.ToLower(newDest) + "/"
		}
	}
	if len(split) > 1 {
		return []byte(newDest + "#" + split[1]), nil
	}
	return []byte(newDest), nil
}

func (r *relLinkTransformer) Close(mdformatter.SourceContext) error { return nil }

type frontMatterTransformer struct {
	localLinksStyle LocalLinksStyle
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

	if f.localLinksStyle.Hugo != nil && f.target.FileName != f.localLinksStyle.Hugo.IndexFileName {
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
	defer errcapture.ExhaustClose(&err, source, "src close")

	if err := os.MkdirAll(filepath.Dir(dst), os.ModePerm); err != nil {
		return err
	}

	destination, err := os.Create(dst)
	if err != nil {
		return errors.Wrap(err, "cpy dest")
	}
	defer errcapture.ExhaustClose(&err, destination, "dst close")

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
	defer errcapture.ExhaustClose(&err, file, "close file")

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
