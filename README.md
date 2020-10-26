# mdox

[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white&style=flat-square)](https://pkg.go.dev/github.com/bwplotka/mdox)
[![Latest Release](https://img.shields.io/github/release/bwplotka/mdox.svg?style=flat-square)](https://github.com/bwplotka/mdox/releases/latest)
[![CI](https://github.com/bwplotka/mdox/workflows/go/badge.svg)](https://github.com/bwplotka/mdox/actions?query=workflow%3Ago)
[![Go Report Card](https://goreportcard.com/badge/github.com/bwplotka/mdox)](https://goreportcard.com/report/github.com/bwplotka/mdox)

CLI toolset for maintaining automated, high quality project documentation and website leveraging markdown and git.

Goal: Allow projects to have self-updating up-to-date documentation available in both markdown (e.g readable from GitHub) and static HTML. Hosted in the same repository as code,
fool-proof and integrated with Pull Requests CI and hosting CD. 

### Features

```bash mdox-gen-exec="mdox --help"
usage: mdox [<flags>] <command> [<args> ...]

Markdown Project Documentation Toolbox.

Flags:
  -h, --help               Show context-sensitive help (also try --help-long and
                           --help-man).
      --version            Show application version.
      --log.level=info     Log filtering level.
      --log.format=logfmt  Log format to use. Possible options: logfmt or json.

Commands:
  help [<command>...]
    Show help.

  fmt <files>...
    Formats given markdown files uniformly following GFM (Github Flavored
    Markdown: https://github.github.com/gfm/).

    Additionally it supports special fenced code directives to autogenerate code
    snippets:

      ```<lang> mdox-gen-exec="<executable + arguments>"

    This directive runs executable with arguments and put its stderr and stdout
    output inside code block content, replacing existing one.

    Example: mdox fmt *.md

  web gen <files>...
    Generate versioned docs
```

### Production Usage

* [Thanos](https://github.com/bwplotka/thanos) (TBD)

## Requirements

* Go 1.14+
* Linux or MacOS

## Installing

```shell
go get github.com/bwplotka/mdox && go mod tidy
```

or via [bingo](github.com/bwplotka/bingo) if want to pin it:

```shell
bingo get -u github.com/bwplotka/mdox
```

Any contributions are welcome! Just use GitHub Issues and Pull Requests as usual.
We follow [Thanos Go coding style](https://thanos.io/contributing/coding-style-guide.md/) guide.

## Initial Author

[@bwplotka](https://bwplotka.dev)
# mdox

[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white&style=flat-square)](https://pkg.go.dev/github.com/bwplotka/mdox) [![Latest Release](https://img.shields.io/github/release/bwplotka/mdox.svg?style=flat-square)](https://github.com/bwplotka/mdox/releases/latest) [![CI](https://github.com/bwplotka/mdox/workflows/go/badge.svg)](https://github.com/bwplotka/mdox/actions?query=workflow%3Ago) [![Go Report Card](https://goreportcard.com/badge/github.com/bwplotka/mdox)](https://goreportcard.com/report/github.com/bwplotka/mdox)

CLI toolset for maintaining automated, high quality project documentation and website leveraging markdown and git.

Goal: Allow projects to have self-updating up-to-date documentation available in both markdown (e.g readable from GitHub) and static HTML. Hosted in the same repository as code, fool-proof and integrated with Pull Requests CI and hosting CD.

### Features

```bash mdox-gen-exec="mdox --help"
usage: mdox [<flags>] <command> [<args> ...]

Markdown Project Documentation Toolbox.

Flags:
  -h, --help               Show context-sensitive help (also try --help-long and
                           --help-man).
      --version            Show application version.
      --log.level=info     Log filtering level.
      --log.format=logfmt  Log format to use. Possible options: logfmt or json.

Commands:
  help [<command>...]
    Show help.

  fmt <files>...
    Formats given markdown files uniformly following GFM (Github Flavored
    Markdown: https://github.github.com/gfm/).

    Additionally it supports special fenced code directives to autogenerate code
    snippets:

      ```<lang> mdox-gen-exec="<executable + arguments>"

    This directive runs executable with arguments and put its stderr and stdout
    output inside code block content, replacing existing one.

    Example: mdox fmt *.md

  web gen <files>...
    Generate versioned docs
```

### Production Usage

* [Thanos](https://github.com/bwplotka/thanos) (TBD)

## Requirements

* Go 1.14+
* Linux or MacOS

## Installing

```shell
go get github.com/bwplotka/mdox && go mod tidy
```

or via [bingo](github.com/bwplotka/bingo) if want to pin it:

```shell
bingo get -u github.com/bwplotka/mdox
```

Any contributions are welcome! Just use GitHub Issues and Pull Requests as usual. We follow [Thanos Go coding style](https://thanos.io/contributing/coding-style-guide.md/) guide.

## Initial Author

[@bwplotka](https://bwplotka.dev)
