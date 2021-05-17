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

	if c.GitIgnore {
		if err = ioutil.WriteFile(filepath.Join(c.OutputDir, ".gitignore"), []byte("*.*"), os.ModePerm); err != nil {
			return err
		}
	}

	var (
		linkTransformer = &relLinkTransformer{outputDir: c.OutputDir, absRelNewPathByFile: map[string]string{}}
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
		files = append(files, path)

		// Copy while preserving structure and tolerating custom mapping.
		// absRelPath is an absolute path, but relatively to input dir (has `/` upfront).
		absRelPath := strings.TrimPrefix(path, c.InputDir)

		target := filepath.Join(c.OutputDir, absRelPath)
		t, ok := firstMatch(absRelPath, c.Transformations)
		if !ok {
			level.Debug(logger).Log("msg", "copying without transformation", "in", path, "absRelPath", absRelPath, "target", target)
			return copyFiles(path, filepath.Join(c.OutputDir, absRelPath))
		}

		var opts []mdformatter.Option
		newAbsRelPath := newTargetAbsRelPath(absRelPath, t)
		if newAbsRelPath != absRelPath {
			linkTransformer.absRelNewPathByFile[path] = newAbsRelPath
		}

		target = filepath.Join(c.OutputDir, newAbsRelPath)
		level.Debug(logger).Log("msg", "copying with transformation", "in", path, "absRelPath", absRelPath, "target", target)
		if err := copyFiles(path, target); err != nil {
			return err
		}

		if t.FrontMatter != nil {
			firstHeader, err := readFirstHeader(path)
			if err != nil {
				return errors.Wrap(err, "read first header")
			}
			opts = append(opts, mdformatter.WithFrontMatterTransformer(&frontMatterTransformer{c: t.FrontMatter, firstHeader: firstHeader}))
		}
		return mdformatter.Format(ctx, logger, []string{target}, opts...)
	}); err != nil {
		return errors.Wrap(err, "walk")
	}

	// Once we did all the changes, change links.
	return mdformatter.Format(ctx, logger, files, mdformatter.WithLinkTransformer(linkTransformer))
}

type relLinkTransformer struct {
	outputDir           string
	absRelNewPathByFile map[string]string
}

func (r *relLinkTransformer) TransformDestination(ctx mdformatter.SourceContext, destination []byte) ([]byte, error) {
	d := string(destination)
	if strings.Contains(d, "://") || filepath.IsAbs(d) {
		return destination, nil
	}
	// TODO(bwplotka): Check if links are outside?

	// absTargetRelPath is an absolute path, but relatively to input dir (has `/` upfront).
	absTargetRelPath := strings.TrimPrefix(ctx.Filepath, r.outputDir)

	if absNewRelPath, ok := r.absRelNewPathByFile[filepath.Join(absTargetRelPath, d)]; ok {
		str, err := filepath.Rel(ctx.Filepath, filepath.Join(r.outputDir, absNewRelPath))
		return []byte(str), err
	}
	return destination, nil
}

func (r *relLinkTransformer) Close(mdformatter.SourceContext) error { return nil }

type frontMatterTransformer struct {
	c *FrontMatterConfig

	// Vars.
	firstHeader string
}

func (f *frontMatterTransformer) TransformFrontMatter(ctx mdformatter.SourceContext, frontMatter map[string]interface{}) ([]byte, error) {
	b := bytes.Buffer{}
	if err := f.c._template.Execute(&b, struct {
		FirstHeader string
		FrontMatter map[string]interface{}
	}{
		FirstHeader: f.firstHeader,
		FrontMatter: frontMatter,
	}); err != nil {
		return nil, err
	}
	m := map[string]interface{}{}
	if err := yaml.Unmarshal(b.Bytes(), m); err != nil {
		return nil, errors.Wrapf(err, "generated template for %v is not a valid yaml", ctx.Filepath)
	}
	return mdformatter.FormatFrontMatter(m), nil
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

// TODO(bwplotka): Use formatter.
func readFirstHeader(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", nil
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		text := scanner.Text()
		if strings.HasPrefix(text, "#") {
			return text, scanner.Err()
		}
	}
	return "<no header found>", nil
}
