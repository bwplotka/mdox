// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"
	"syscall"
	"time"

	"github.com/bwplotka/mdox/pkg/clilog"
	"github.com/bwplotka/mdox/pkg/extkingpin"
	"github.com/bwplotka/mdox/pkg/mdformatter"
	"github.com/bwplotka/mdox/pkg/mdformatter/linktransformer"
	"github.com/bwplotka/mdox/pkg/mdformatter/mdgen"
	"github.com/bwplotka/mdox/pkg/transform"
	"github.com/bwplotka/mdox/pkg/version"
	"github.com/charmbracelet/glamour"
	"github.com/efficientgo/tools/core/pkg/errcapture"
	"github.com/efficientgo/tools/core/pkg/logerrcapture"
	extflag "github.com/efficientgo/tools/extkingpin"
	"github.com/felixge/fgprof"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/oklog/run"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/expfmt"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	logFormatLogfmt = "logfmt"
	logFormatJson   = "json"
	logFormatCLILog = "clilog"
)

type mdoxMetrics struct {
	reg *prometheus.Registry
}

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
	switch logFormat {
	case logFormatJson:
		return level.NewFilter(log.NewJSONLogger(log.NewSyncWriter(os.Stderr)), lvl)
	case logFormatLogfmt:
		return level.NewFilter(log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr)), lvl)
	case logFormatCLILog:
		fallthrough
	default:
		return level.NewFilter(clilog.New(log.NewSyncWriter(os.Stderr)), lvl)
	}
}

func main() {
	app := extkingpin.NewApp(kingpin.New(filepath.Base(os.Args[0]), `Markdown Project Documentation Toolbox.`).Version(version.Version))
	logLevel := app.Flag("log.level", "Log filtering level.").
		Default("info").Enum("error", "warn", "info", "debug")
	logFormat := app.Flag("log.format", "Log format to use.").
		Default(logFormatCLILog).Enum(logFormatLogfmt, logFormatJson, logFormatCLILog)
	// Profiling and metrics.
	profilesPath := app.Flag("debug.profiles", "Path to which CPU and heap profiles are saved").Hidden().String()
	metrics := app.Flag("metrics", "Enable metrics and view them at https://localhost:9091/metrics").Hidden().Bool()

	m := &mdoxMetrics{}

	ctx, cancel := context.WithCancel(context.Background())
	registerFmt(ctx, app, m)
	registerTransform(ctx, app)

	cmd, runner := app.Parse()
	logger := setupLogger(*logLevel, *logFormat)

	if *profilesPath != "" {
		finalize, err := snapshotProfiles(*profilesPath)
		if err != nil {
			level.Error(logger).Log("err", errors.Wrapf(err, "%s profiles init failed", cmd))
			os.Exit(1)
		}
		defer logerrcapture.Do(logger, finalize, "profiles")
	}

	var g run.Group
	g.Add(func() error {
		// TODO(bwplotka): Move to customized better setup function.
		return runner(ctx, logger)
	}, func(err error) {
		cancel()
	})

	if *metrics {
		srv := &http.Server{Addr: ":9091"}
		m.reg = prometheus.NewRegistry()

		g.Add(func() error {
			http.Handle("/metrics", promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{}))
			return srv.ListenAndServe()
		}, func(err error) {
			_ = srv.Shutdown(ctx)
			cancel()
		})
	}

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
		if *logLevel == "debug" {
			// Use %+v for github.com/pkg/errors error to print with stack.
			level.Error(logger).Log("err", fmt.Sprintf("%+v", errors.Wrapf(err, "%s command failed", cmd)))
			os.Exit(1)
		}
		level.Error(logger).Log("err", errors.Wrapf(err, "%s command failed", cmd))
		os.Exit(1)
	}

}

func snapshotProfiles(dir string) (func() error, error) {
	now := time.Now().UTC()
	if err := os.MkdirAll(filepath.Join(dir, strings.ReplaceAll(now.Format(time.UnixDate), " ", "_")), os.ModePerm); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, strings.ReplaceAll(now.Format(time.UnixDate), " ", "_"), "fgprof.pb.gz"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.ModePerm)
	if err != nil {
		return nil, err
	}

	m, err := os.OpenFile(filepath.Join(dir, strings.ReplaceAll(now.Format(time.UnixDate), " ", "_"), "memprof.pb.gz"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.ModePerm)
	if err != nil {
		return nil, err
	}
	runtime.GC()

	if err := pprof.WriteHeapProfile(m); err != nil {
		return nil, err
	}

	fgFunc := fgprof.Start(f, fgprof.FormatPprof)

	return func() (err error) {
		defer errcapture.Do(&err, f.Close, "close")
		return fgFunc()
	}, nil
}

func (m *mdoxMetrics) Print() error {
	mfs, err := m.reg.Gather()
	if err != nil {
		return err
	}

	enc := expfmt.NewEncoder(os.Stdout, expfmt.FmtProtoText)

	for _, mf := range mfs {
		if err := enc.Encode(mf); err != nil {
			return err
		}
	}
	return nil
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

func registerFmt(_ context.Context, app *extkingpin.App, m *mdoxMetrics) {
	cmd := app.Command("fmt", "Formats in-place given markdown files uniformly following GFM (Github Flavored Markdown: https://github.github.com/gfm/). Example: mdox fmt *.md")
	files := cmd.Arg("files", "Markdown file(s) to process.").Required().ExistingFiles()
	checkOnly := cmd.Flag("check", "If true, fmt will not modify the given files, instead it will fail if files needs formatting").Bool()

	disableGenCodeBlocksDirectives := cmd.Flag("code.disable-directives", `If false, fmt will parse custom fenced code directives prefixed with 'mdox-gen' to autogenerate code snippets. For example:
	`+"```"+`<lang> mdox-exec="<executable + arguments>"
This directive runs executable with arguments and put its stderr and stdout output inside code block content, replacing existing one.`).Bool()
	anchorDir := cmd.Flag("anchor-dir", "Anchor directory for all transformers. PWD is used if flag is not specified.").ExistingDir()
	linksLocalizeForAddress := cmd.Flag("links.localize.address-regex", "If specified, all HTTP(s) links that target a domain and path matching given regexp will be transformed to relative to anchor dir path (if exists)."+
		"Absolute path links will be converted to relative links to anchor dir as well.").Regexp()
	// TODO(bwplotka): Add cache in file?
	linksValidateEnabled := cmd.Flag("links.validate", "If true, all links will be validated").Short('l').Bool()
	linksValidateConfig := extflag.RegisterPathOrContent(cmd, "links.validate.config", "YAML file for skipping link check, with spec defined in github.com/bwplotka/mdox/pkg/linktransformer.ValidatorConfig", extflag.WithEnvSubstitution())

	cmd.Run(func(ctx context.Context, logger log.Logger) (err error) {
		if m.reg != nil {
			defer logerrcapture.Do(logger, m.Print, "print")
		}

		var opts []mdformatter.Option
		if !*disableGenCodeBlocksDirectives {
			opts = append(opts, mdformatter.WithCodeBlockTransformer(mdgen.NewCodeBlockTransformer()))
		}
		if len(*files) == 0 {
			return errors.New("no files to format")
		}

		for i := range *files {
			(*files)[i], err = filepath.Abs((*files)[i])
			if err != nil {
				return err
			}
		}

		anchorDir, err := validateAnchorDir(*anchorDir, *files)
		if err != nil {
			return err
		}

		var linkTr []mdformatter.LinkTransformer
		if *linksValidateEnabled {
			validateConfigContent, err := linksValidateConfig.Content()
			if err != nil {
				return err
			}
			v, err := linktransformer.NewValidator(ctx, logger, validateConfigContent, anchorDir, m.reg)
			if err != nil {
				return err
			}
			linkTr = append(linkTr, v)
		}
		if *linksLocalizeForAddress != nil {
			linkTr = append(linkTr, linktransformer.NewLocalizer(logger, *linksLocalizeForAddress, anchorDir))
		}

		if len(linkTr) > 0 {
			opts = append(opts, mdformatter.WithLinkTransformer(linktransformer.NewChain(linkTr...)))
		}

		if *checkOnly {
			diff, err := mdformatter.IsFormatted(ctx, logger, *files, m.reg, opts...)
			if err != nil {
				return err
			}
			if len(diff) == 0 {
				return nil
			}
			grender, err := glamour.NewTermRenderer(
				glamour.WithAutoStyle(),
				glamour.WithWordWrap(100),
			)
			if err != nil {
				return err
			}
			diffOut, err := grender.Render("\n```diff\n" + diff.String() + "\n```\n")
			if err != nil {
				return err
			}
			return errors.Errorf("files not formatted: %v", diffOut)

		}
		return mdformatter.Format(ctx, logger, *files, m.reg, opts...)
	})
}

// validateAnchorDir returns validated anchor dir against files provided.
func validateAnchorDir(anchorDir string, files []string) (_ string, err error) {
	if anchorDir == "" {
		anchorDir, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}

	anchorDir, err = filepath.Abs(anchorDir)
	if err != nil {
		return "", err
	}

	// Check if provided files are within anchorDir way.
	for _, f := range files {
		if !strings.HasPrefix(f, anchorDir) {
			return "", errors.Errorf("anchorDir %q is not in path of provided file %q", anchorDir, f)
		}
	}
	return anchorDir, nil
}

func registerTransform(_ context.Context, app *extkingpin.App) {
	cmd := app.Command("transform", "Transform markdown files in various ways. For example pre process markdown files to allow it for use for popular static HTML websites based on markdown source code and front matter options.")
	cfg := extflag.RegisterPathOrContent(cmd, "config", "Path to the YAML file with spec defined in github.com/bwplotka/mdox/pkg/transform.Config", extflag.WithEnvSubstitution())
	cmd.Run(func(ctx context.Context, logger log.Logger) error {
		validateConfig, err := cfg.Content()
		if err != nil {
			return err
		}
		return transform.Dir(ctx, logger, validateConfig)
	})
}
