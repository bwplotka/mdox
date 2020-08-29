package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/oklog/run"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
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

// TODO(bwplotka): Move to separate repo once it's more generic. Document, version etc.
func main() {
	app := extkingpin.NewApp(kingpin.New(filepath.Base(os.Args[0]), `Markdown Project Documentation Toolbox.

Features:
* Prepare versioned docs formatted for static website. (TBD)
* Generate docs from configuration structs and flags (TBD)
* Format links to work well for website as well as raw markdown.
* Check links.

`).Version("yolo"))
	logLevel := app.Flag("log.level", "Log filtering level.").
		Default("info").Enum("error", "warn", "info", "debug")
	logFormat := app.Flag("log.format", "Log format to use. Possible options: logfmt or json.").
		Default(logFormatLogfmt).Enum(logFormatLogfmt, logFormatJson)

	ctx, cancel := context.WithCancel(context.Background())
	registerFmt(ctx, app)

	cmd, runner := app.Parse()
	logger := setupLogger(*logLevel, *logFormat)

	var g run.Group
	g.Add(func() error {
		// TODO(bwplotka): Move to customised better setup function.
		return runner(nil, logger, nil, nil, nil, false)
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

func registerFmt(ctx context.Context, app *extkingpin.App) {
	cmd := app.Command("fmt", "Format Markdown docs.")
	files := app.Arg("files", "Markdown file(s) to process.").Required().ExistingFiles()
	relLinksRes := app.Flag("links.rel-refs-regexp", "Regexp(es) for link references that should be relative link in order to have them work across different domains or platforms").Required().Strings()
	// TODO: Output.
	cmd.Setup(func(_ *run.Group, logger log.Logger, _ *prometheus.Registry, _ opentracing.Tracer, _ <-chan struct{}, _ bool) error {
		root, err := os.Getwd()
		if err != nil {
			return err
		}
		return mdox.Format(ctx, logger, *files)
	})
}

func registerHugoFmt(ctx context.Context, app *extkingpin.App) {
	cmd := app.Command("fmt", "Format Markdown docs.")
	files := app.Arg("files", "Markdown file(s) to process.").Required().ExistingFiles()
	relLinksRes := app.Flag("links.rel-refs-regexp", "Regexp(es) for link references that should be relative link in order to have them work across different domains or platforms").Required().Strings()
	// TODO: Output.
	cmd.Setup(func(_ *run.Group, logger log.Logger, _ *prometheus.Registry, _ opentracing.Tracer, _ <-chan struct{}, _ bool) error {
		root, err := os.Getwd()
		if err != nil {
			return err
		}
		return mdox.Format(ctx, logger, root, *files, *relLinksRes)
	})
}
