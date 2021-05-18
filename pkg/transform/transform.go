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

	_, err = os.Stat(c.OutputDir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if !os.IsNotExist(err) {
		if err := os.RemoveAll(c.OutputDir); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(c.OutputDir, os.ModePerm); err != nil {
		return err
	}

	if c.GitIgnored {
		if err = ioutil.WriteFile(filepath.Join(c.OutputDir, ".gitignore"), []byte("*.*"), os.ModePerm); err != nil {
			return err
		}
	}

	var (
		linkTransformer = &relLinkTransformer{outputDir: c.OutputDir, newAbsRelPathByOldAbsRelPath: map[string]string{}}
		files           []string
	)

	// Move markdown files, preserving dir structure to output while preprocessing files.
	if err := filepath.Walk(c.InputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || path == c.InputDir {
			return nil
		}

		// Copy while preserving structure and tolerating custom mapping.
		// absRelPath is an absolute path, but relatively to input dir (has `/` upfront).
		absRelPath := strings.TrimPrefix(path, c.InputDir)

		target := filepath.Join(c.OutputDir, absRelPath)
		defer func() {
			files = append(files, target)
		}()

		t, ok := firstMatch(absRelPath, c.Transformations)
		if !ok {
			level.Debug(logger).Log("msg", "copying without transformation", "in", path, "absRelPath", absRelPath, "target", target)
			return copyFiles(path, filepath.Join(c.OutputDir, absRelPath))
		}

		var opts []mdformatter.Option
		newAbsRelPath := newTargetAbsRelPath(absRelPath, t)
		if newAbsRelPath != absRelPath {
			linkTransformer.newAbsRelPathByOldAbsRelPath[absRelPath] = newAbsRelPath
		}

		target = filepath.Join(c.OutputDir, newAbsRelPath)
		level.Debug(logger).Log("msg", "copying with transformation", "in", path, "absRelPath", absRelPath, "target", target)
		if err := copyFiles(path, target); err != nil {
			return err
		}

		if t.FrontMatter != nil {
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
				c: t.FrontMatter,
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
		return mdformatter.Format(ctx, logger, []string{target}, opts...)
	}); err != nil {
		return errors.Wrap(err, "walk")
	}

	// Once we did all the changes, change links.
	return mdformatter.Format(ctx, logger, files, mdformatter.WithLinkTransformer(linkTransformer))
}

type relLinkTransformer struct {
	localLinksStyle LinksStyle

	outputDir                    string
	newAbsRelPathByOldAbsRelPath map[string]string
}

func (r *relLinkTransformer) TransformDestination(ctx mdformatter.SourceContext, destination []byte) ([]byte, error) {
	split := strings.Split(string(destination), "#")
	dest := split[0]
	if strings.Contains(dest, "://") || filepath.IsAbs(dest) || strings.HasPrefix(string(destination), "#") {
		return destination, nil
	}
	// TODO(bwplotka): Check if links are outside?
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

	// Do we changed?
	change := ""
	for n, old := range r.newAbsRelPathByOldAbsRelPath {
		if old != currentAbsRelPath {
			continue
		}
		c, err := filepath.Rel(filepath.Dir(old), filepath.Dir(n))
		if err != nil {
			return nil, err
		}
		change = c
		break
	}

	adjustedAbsRelDir := filepath.Join(currentAbsRelDir, change)
	adjustedAbsRelDest := filepath.Join(adjustedAbsRelDir, dest)

	// Does the link points to something that changed?
	if absNewRelPath, ok := r.newAbsRelPathByOldAbsRelPath[adjustedAbsRelDest]; ok {
		adjustedAbsRelDest = absNewRelPath
	}

	newDest, err := filepath.Rel(currentAbsRelDir, adjustedAbsRelDest)
	if err != nil {
		return nil, err
	}

	if newDest == "." {
		newDest = ""
	} else if r.localLinksStyle == Hugo {
		// Because all links are normally files, in Hugo those are literally URL paths (kind of "dirs").
		// This is why we need to add ../ to them.
		newDest = filepath.Join("..", newDest)

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
	if f.localLinksStyle == Hugo {
		if _, ok := frontMatter["slug"]; !ok {
			frontMatter["slug"] = "{{ .Target.FileName }}"
		}
	}

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

func newTargetAbsRelPath(absRelPath string, tr *TransformationConfig) string {
	if tr.Path == "" {
		return absRelPath
	}

	if filepath.IsAbs(tr.Path) {
		return tr.Path
	}

	return filepath.Join(filepath.Dir(absRelPath), tr.Path)
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
	return "", nil, errors.New("No header found")
}
