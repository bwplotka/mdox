// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/bwplotka/mdox/pkg/extkingpin"
	"github.com/bwplotka/mdox/pkg/mdformatter"
	"github.com/bwplotka/mdox/pkg/mdgen"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/oklog/run"
	"github.com/pkg/errors"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	logFormatLogfmt = "logfmt"
	logFormatJson   = "json"
)

func setupLogger(logLevel, logFormat string) log.Logger {
	var lvl level.Option
	switch logLevel {
	case "error":
		lvl = level.AllowError()
	case "warn":
		lvl = level.AllowWarn()
	case "info":
		lvl = level.AllowInfo()
	case "debug":
		lvl = level.AllowDebug()
	default:
		panic("unexpected log level")
	}
	logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
	if logFormat == logFormatJson {
		logger = log.NewJSONLogger(log.NewSyncWriter(os.Stderr))
	}
	logger = level.NewFilter(logger, lvl)
	return log.With(logger, "ts", log.DefaultTimestampUTC, "caller", log.DefaultCaller)
}

func main() {
	app := extkingpin.NewApp(kingpin.New(filepath.Base(os.Args[0]), `Markdown Project Documentation Toolbox.`).Version("v0.0.0"))
	logLevel := app.Flag("log.level", "Log filtering level.").
		Default("info").Enum("error", "warn", "info", "debug")
	logFormat := app.Flag("log.format", "Log format to use. Possible options: logfmt or json.").
		Default(logFormatLogfmt).Enum(logFormatLogfmt, logFormatJson)

	ctx, cancel := context.WithCancel(context.Background())
	registerFmt(ctx, app)
	registerWeb(ctx, app)

	cmd, runner := app.Parse()
	logger := setupLogger(*logLevel, *logFormat)

	var g run.Group
	g.Add(func() error {
		// TODO(bwplotka): Move to customized better setup function.
		return runner(ctx, logger)
	}, func(err error) {
		cancel()
	})

	// Listen for termination signals.
	{
		cancel := make(chan struct{})
		g.Add(func() error {
			return interrupt(logger, cancel)
		}, func(error) {
			close(cancel)
		})
	}

	if err := g.Run(); err != nil {
		// Use %+v for github.com/pkg/errors error to print with stack.
		level.Error(logger).Log("err", fmt.Sprintf("%+v", errors.Wrapf(err, "%s command failed", cmd)))
		os.Exit(1)
	}
	level.Info(logger).Log("msg", "exiting")
}

func interrupt(logger log.Logger, cancel <-chan struct{}) error {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	select {
	case s := <-c:
		level.Info(logger).Log("msg", "caught signal. Exiting.", "signal", s)
		return nil
	case <-cancel:
		return errors.New("canceled")
	}
}

func registerFmt(_ context.Context, app *extkingpin.App) {
	cmd := app.Command("fmt", `
Formats given markdown files uniformly following GFM (Github Flavored Markdown: https://github.github.com/gfm/).

Additionally it supports special fenced code directives to autogenerate code snippets:

	`+"```"+`<lang> mdox-gen-exec="<executable + arguments>"

This directive runs executable with arguments and put its stderr and stdout output inside code block content, replacing existing one.

Example: mdox fmt *.md
`)
	files := cmd.Arg("files", "Markdown file(s) to process.").Required().ExistingFiles()
	cmd.Run(func(ctx context.Context, logger log.Logger) error {
		return mdformatter.Format(ctx, logger, *files, mdformatter.WithCodeBlockTransformer(mdgen.NewCodeBlockTransformer()))
	})
}

func registerWeb(_ context.Context, app *extkingpin.App) {
	cmd := app.Command("web", "Tools for generating static HTML website based on https://gohugo.io/ on every PR with preview")
	genCmd := cmd.Command("gen", "Generate versioned docs")

	_ = genCmd.Arg("files", "Markdown file(s) to process.").Required().ExistingFiles()

	// TODO(bwplotka): Generate versioned docs used for Hugo. Open question: Hugo specific? How to adjust links? Should fmt and this be
	// the same?
	cmd.Run(func(ctx context.Context, logger log.Logger) error {
		return errors.New("not implemented")
	})
}
