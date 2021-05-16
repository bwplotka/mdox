package web

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/bwplotka/mdox/pkg/mdformatter"
	"github.com/bwplotka/mdox/pkg/mdformatter/linktransformer"
	"github.com/efficientgo/tools/pkg/errcapture"
	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
)

func ProcessGitHubDir(ctx context.Context, logger log.Logger, configFile string) error {
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
	if err = ioutil.WriteFile(filepath.Join(c.OutputDir, ".gitignore"), []byte("*.*"), os.ModePerm); err != nil {
		return err
	}

	// Move markdown files, preserving dir structure to output while preprocessing files.
	if err := filepath.Walk(c.ContentDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == c.ContentDir {
			return nil
		}

		// Copy while preserving structure and tolerating custom mapping.
		relPath := strings.TrimPrefix(path, c.ContentDir+"/")
		target := filepath.Join(c.OutputDir, relPath)

		if t, ok := greaterMatchPath(c.FileMapping, relPath); ok {
			target = filepath.Join(c.OutputDir, t)
		}
		fmt.Println("t", target)

		if info.IsDir() {
			return os.MkdirAll(target, os.ModePerm)
		}

		if err := copyFiles(path, target); err != nil {
			return err
		}

		fmc, ok := greaterMatchFrontMatterConfig(c.FrontMatter, relPath)
		if !ok{
			return nil
		}

		fmc._r.Execute()

		return mdformatter.Format(
			ctx,
			logger,
			[]string{target},
			mdformatter.WithLinkTransformer(linktransformer.NewChain(linkTr...)))
			mdformatter.WithFrontMatterTransformer()
		)



	}); err != nil {
		return errors.Wrap(err, "walk")
	}
	return nil
}

type FrontMatterVars struct {
	FirstHeader string
	RelativePath string
}


// TODO(bwplotka): Support advanced paths (../)
// TODO: Give relative!
func greaterMatchPath(mapping map[string]string, relPath string) (string, bool) {
	pathSplit := strings.Split(relPath, "/")
	if t, ok := mapping["/"+relPath]; ok {
		return filepath.Join(filepath.Dir(relPath), t), true
	}

	for i := 0; i < len(pathSplit); i++ {
		if t, ok := mapping[filepath.Join(pathSplit[i:]...)]; ok {
			return filepath.Join(filepath.Dir(relPath), t), true
		}
	}
	if t, ok := mapping["/"]; ok {
		return filepath.Join(filepath.Dir(relPath), t), true
	}
	return "", false
}

func greaterMatchFrontMatterConfig(mapping map[string]*FrontMatterConfig, relPath string) (*FrontMatterConfig, bool) {
	pathSplit := strings.Split(relPath, "/")
	if t, ok := mapping["/"+relPath]; ok {
		return t, true
	}

	for i := 0; i < len(pathSplit); i++ {
		if t, ok := mapping[filepath.Join(pathSplit[i:]...)]; ok {
			return t, true
		}
	}
	if t, ok := mapping["/"]; ok {
		return t, true
	}
	return nil, false
}

func copyFiles(src, dst string) (err error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return errors.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer errcapture.Close(&err, source, "src close")

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer errcapture.Close(&err, destination, "dst close")

	_, err = io.Copy(destination, source)
	return err
}

