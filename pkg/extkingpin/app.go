// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

// Taken from Thanos project.
//
// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.
package extkingpin

import (
	"context"
	"fmt"
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/go-kit/log"
)

type FlagClause interface {
	Flag(name, help string) *kingpin.FlagClause
}

type ArgClause interface {
	Arg(name, help string) *kingpin.ArgClause
}

type Run func(ctx context.Context, logger log.Logger) error

type AppClause interface {
	FlagClause
	ArgClause
	Command(cmd string, help string) AppClause
	Flags() []*kingpin.FlagModel
	Run(r Run)
}

// App is a wrapper around kingping.Application for easier use.
type App struct {
	FlagClause
	app  *kingpin.Application
	runs map[string]Run
}

// NewApp returns new App.
func NewApp(app *kingpin.Application) *App {
	app.HelpFlag.Short('h')
	return &App{
		app:        app,
		FlagClause: app,
		runs:       map[string]Run{},
	}
}

func (a *App) Parse() (cmd string, runner Run) {
	cmd, err := a.app.Parse(os.Args[1:])
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, fmt.Errorf("error parsing commandline arguments: %v: %w", os.Args, err))
		a.app.Usage(os.Args[1:])
		os.Exit(2)
	}
	return cmd, a.runs[cmd]
}

func (a *App) Command(cmd string, help string) AppClause {
	c := a.app.Command(cmd, help)
	return &appClause{
		c:          c,
		FlagClause: c,
		ArgClause:  c,
		runs:       a.runs,
		prefix:     cmd,
	}
}

type appClause struct {
	c *kingpin.CmdClause

	FlagClause
	ArgClause
	runs   map[string]Run
	prefix string
}

func (a *appClause) Command(cmd string, help string) AppClause {
	c := a.c.Command(cmd, help)
	return &appClause{
		c:          c,
		FlagClause: c,
		ArgClause:  c,
		runs:       a.runs,
		prefix:     a.prefix + " " + cmd,
	}
}

func (a *appClause) Run(s Run) {
	a.runs[a.prefix] = s
}

func (a *appClause) Flags() []*kingpin.FlagModel {
	return a.c.Model().Flags
}
