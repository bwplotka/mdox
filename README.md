# mdox

[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white&style=flat-square)](https://pkg.go.dev/github.com/bwplotka/mdox) [![Latest Release](https://img.shields.io/github/release/bwplotka/mdox.svg?style=flat-square)](https://github.com/bwplotka/mdox/releases/latest) [![CI](https://github.com/bwplotka/mdox/workflows/go/badge.svg)](https://github.com/bwplotka/mdox/actions?query=workflow%3Ago) [![Go Report Card](https://goreportcard.com/badge/github.com/bwplotka/mdox)](https://goreportcard.com/report/github.com/bwplotka/mdox) [![Slack](https://img.shields.io/badge/join%20slack-%23mdox-brightgreen.svg)](https://cloud-native.slack.com/archives/mdox)

`mdox` (spelled as `md docs`) is a CLI for maintaining automated, high-quality project documentation and website leveraging [Github Flavored Markdown](https://github.github.com/gfm/) and git.

This project can be used both as CLI as well as a library.

## Goals

Allow projects to have self-updating up-to-date documentation available in both markdown (e.g readable from GitHub) and static HTML. Hosted in the same repository as code and integrated with Pull Requests CI, hosting CD, and code generation.

## Features

* Enhanced and consistent formatting for markdown files in [GFM](https://github.github.com/gfm/) format, focused on readability.
* Auto generation of code block content based on `mdox-exec` directives (see [#code-generation](#code-generation)). Useful for:
  * Generating help output from CLI --help
  * Generating example YAML from Go configuration struct (+comments)
* Robust and fast relative and remote link checking. (see [#link-validation-configuration](#link-validation-configuration))
* Website integration:
  * "Localizing" links to relative docs if specified (useful for multi-domain websites or multi-version doc). (see [#link-localization](#link-localization))
    * This allows smooth integration with static document websites like [Docusaurus](https://docusaurus.io/) or [hugo](https://gohugo.io) based themes!
  * Flexible pre-processing allowing easy to use GitHub experience as well as website. (see [#transform-usage](#transformation))
* Allows profiling(using [fgprof](https://github.com/felixge/fgprof)) and exports metrics(saves to file in [OpenMetrics](https://openmetrics.io/) format) for easy debugging

## Usage

### Formatting and Link Checking

Just run `mdox fmt` and pass markdown files (or glob matching those).

For example, this README is formatted by the CI on every PR using [`mdox fmt -l *.md` command](https://github.com/bwplotka/mdox/blob/9e183714070f464b1ef089da3df8048aff1abeda/Makefile#L59).

```bash mdox-exec="mdox fmt --help"
usage: mdox fmt [<flags>] <files>...

Formats in-place given markdown files uniformly following GFM (Github Flavored
Markdown: https://github.github.com/gfm/). Example: mdox fmt *.md

Flags:
  -h, --help                     Show context-sensitive help (also try
                                 --help-long and --help-man).
      --version                  Show application version.
      --log.level=info           Log filtering level.
      --log.format=clilog        Log format to use.
      --profiles.path=PROFILES.PATH  
                                 Path to directory where CPU and heap profiles
                                 will be saved; If empty, no profiling will be
                                 enabled.
      --metrics.path=METRICS.PATH  
                                 Path to directory where metrics are saved in
                                 OpenMetrics format; If empty, no metrics will
                                 be saved.
      --check                    If true, fmt will not modify the given files,
                                 instead it will fail if files needs formatting
      --soft-wraps               If true, fmt will preserve soft line breaks for
                                 given files
      --code.disable-directives  If false, fmt will parse custom fenced code
                                 directives prefixed with 'mdox-gen' to
                                 autogenerate code snippets. For example:
                                 
                                   ```<lang> mdox-exec="<executable + arguments>"
                                 
                                 This directive runs executable with arguments
                                 and put its stderr and stdout output inside
                                 code block content, replacing existing one.
      --anchor-dir=ANCHOR-DIR    Anchor directory for all transformers. PWD is
                                 used if flag is not specified.
      --links.localize.address-regex=LINKS.LOCALIZE.ADDRESS-REGEX  
                                 If specified, all HTTP(s) links that target a
                                 domain and path matching given regexp will be
                                 transformed to relative to anchor dir path (if
                                 exists).Absolute path links will be converted
                                 to relative links to anchor dir as well.
  -l, --links.validate           If true, all links will be validated
      --links.validate.config-file=<file-path>  
                                 Path to YAML file for skipping link check, with
                                 spec defined in
                                 github.com/bwplotka/mdox/pkg/linktransformer.ValidatorConfig
      --links.validate.config=<content>  
                                 Alternative to 'links.validate.config-file'
                                 flag (mutually exclusive). Content of YAML file
                                 for skipping link check, with spec defined in
                                 github.com/bwplotka/mdox/pkg/linktransformer.ValidatorConfig

Args:
  <files>  Markdown file(s) to process.

```

#### Code Generation

It's not uncommon that documentation is explaining code or configuration snippets. One of the challenges of such documentation is keeping it up to date. This is where `mdox` code block directives comes handy! To ensure mdox will auto update code snippet add `mdox-exec="<whatever command you want take output from>"` after language directive on code block.

For example this Readme contains `mdox --help` which is has to be auto generated on every PR:

```markdown
``` bash mdox-exec="mdox fmt --help"
...
```

This also enables auto updating snippets of code in code blocks using tools like `sed`. For example, below code block directive will auto update and insert lines 3 to 6 from main.go into code block.

```markdown
```go mdox-exec="sed -n '3,6p' main.go"
...
```

Some commands might have non-zero exit codes. mdox will fail commands in such cases(otherwise errors might get formatted into markdown) but the expected exit code can also be passed as a code block directive! For example, below code block executes `go --help` which has 2 as its exit code,

```markdown
```go mdox-exec="go --help" mdox-expect-exit-code=2
...
```

You can disable this feature by specifying `--code.disable-directives`

#### Link Validation Configuration

By default, mdox checks all links (both relative and remote) in passed markdown files! However, in some cases, link checks might fail even when links are working, such as with rate limiting, Cloudflare protections, or something else(like localhost links in docs).

This might lead to failed CI checks which aren't desirable. So, you can use the `links.validate.config-file` flag to pass in YAML configuration file for selective link checking using special validators and link regexes.

For example,

```yaml mdox-exec="cat examples/.mdox.validate.yaml"
version: 1

validators:
  - regex: '(^http[s]?:\/\/)(www\.)?(github\.com\/)bwplotka\/mdox(\/pull\/|\/issues\/)'
    type: 'githubPullsIssues'

  - regex: 'localhost'
    type: 'ignore'

  - regex: 'thanos\.io'
    type: 'roundtrip'

```

As seen above, mdox supports passing an array of link validators with types and regexes. There are three types of validators,

* `ignore`: This type of validator makes sure that `mdox` does not check links with provided regex. This is the most common use case.
* `githubPullsIssues`: This is a smart validator which only accepts a specific type of regex of the form `(^http[s]?:\/\/)(www\.)?(github\.com\/){ORG}\/{REPO}(\/pull\/|\/issues\/)`. It performs smart validation on GitHub PR and issues links, by fetching GitHub API to get the latest pull/issue number and matching regex. This makes sure that mdox doesn't get rate limited by GitHub, even when checking a large number of GitHub links(which is pretty common in documentation)!
* `roundtrip`: All links are checked with the roundtrip validator by default(no need for including into config explicitly) which means that each link is visited and fails if http status code is not 200(even after retries).

Relative link checking *is not* affected by this configuration, as it is expected that such links will work.

YAML can be passed in directly as well using `links.validate.config` flag! For more details [go.dev reference](https://pkg.go.dev/github.com/bwplotka/mdox) or [Go struct](https://github.com/bwplotka/mdox/blob/main/pkg/mdformatter/linktransformer/config.go).

### Link localization

It is expected for documentation to contain remote links to the project website. However, in such cases, it creates problems for multi-version docs or multi-domain websites (links would need to be updated for each version which is cumbersome). Also, it would not be navigatable locally or through GitHub(would always redirect to the website) and requires additional link checking.

This is where the `links.localize.address-regex` flag comes in handy!

It ensures that all HTTP(s) links that target a domain and path matching given regex will be transformed by `mdox` to relative links which are relative to anchor dir path (if exists). Also, all absolute path links will be converted to relative links to anchor dir as well.

So passing in regex such as `--links.localize.address-regex="https:\/\/example\.\/.*` will allow mdox to transform links like `https://example.com/getting-started.md/` to simply `getting-started.md`.

### Transformation

mdox allows various types of markdown file transformation which are useful for website pre-processing and is often required when using static site generators like Hugo. It helps in generating front/backmatter, renaming and moving files, and converts links to work on websites.

Just run `mdox transform --config-file=.mdox.yaml` and pass in YAML configuration.

```bash mdox-exec="mdox transform --help"
usage: mdox transform [<flags>]

Transform markdown files in various ways. For example pre process markdown files
to allow it for use for popular static HTML websites based on markdown source
code and front matter options.

Flags:
  -h, --help                     Show context-sensitive help (also try
                                 --help-long and --help-man).
      --version                  Show application version.
      --log.level=info           Log filtering level.
      --log.format=clilog        Log format to use.
      --profiles.path=PROFILES.PATH  
                                 Path to directory where CPU and heap profiles
                                 will be saved; If empty, no profiling will be
                                 enabled.
      --metrics.path=METRICS.PATH  
                                 Path to directory where metrics are saved in
                                 OpenMetrics format; If empty, no metrics will
                                 be saved.
      --config-file=<file-path>  Path to Path to the YAML file with spec defined
                                 in
                                 github.com/bwplotka/mdox/pkg/transform.Config
      --config=<content>         Alternative to 'config-file' flag (mutually
                                 exclusive). Content of Path to the YAML file
                                 with spec defined in
                                 github.com/bwplotka/mdox/pkg/transform.Config

```

For example,

```yaml mdox-exec="cat examples/.mdox.yaml"
version: 1

inputDir: "docs"
outputDir: "website/docs-pre-processed/tip"
extraInputGlobs:
  - "CHANGELOG.md"
  - "static"

gitIgnored: true
localLinksStyle:
  hugo:
    indexFileName: "_index.md"

transformations:

  - glob: "../CHANGELOG.md"
    path: /thanos/CHANGELOG.md
    popHeader: true
    frontMatter:
      template: |
        title: "{{ .Origin.FirstHeader }}"
        type: docs
        lastmod: "{{ .Origin.LastMod }}"
    backMatter:
      template: |
        Found a typo, inconsistency or missing information in our docs?
        Help us to improve [Thanos](https://thanos.io) documentation by proposing a fix [on GitHub here](https://github.com/thanos-io/thanos/edit/main/{{ .Origin.Filename }}) :heart:

  - glob: "getting-started.md"
    path: /thanos/getting-started.md
    frontMatter:
      template: |
        type: docs
        title: "{{ .Origin.FirstHeader }}"
        lastmod: "{{ .Origin.LastMod }}"
        slug: "{{ .Target.FileName }}"

  - glob: "../static/**"
    path: /favicons/**
```

As seen above,

* `inputDir`: It's a relative (to PWD) path that assumes input directory for markdown files and assets.
* `outputDir`: It's a relative (to PWD) output directory where you can expect all files to land in. Typically that can be a `content` dir which Hugo uses as an input.
* `extraInputGlobs`: It allows you to bring files from outside of `inputDir`.
* `linkPrefixForNonMarkdownResources`: It specifies the link to be glued onto relative links which don't point to markdown or image files.
* `gitIgnored`: It specifies whether `outputDir` should be git-ignored.
* `localLinksStyle`: It sets the linking style to be applied. If empty, mdox assumes default style.

* `transformation`: Array of transformations to apply for files in `inputDir` and `extraInputGlobs`. This consists of,
  * `glob`: It is matched against the relative path of the file in the `inputDir` using https://github.com/gobwas/glob.
  * `path`: It is an optional different path for the file to be moved into. If not specified, the file will be moved to the exact same position as it is in `inputDir`.
  * `popHeader`: If set to true, it pops the first header of md file. True by default for files in the root of `inputDir`
  * `frontMatter`: Optional template for constructing frontmatter of markdown file.
  * `backMatter`: Optional template for constructing backmatter of markdown file(content appended to end like edit links)

YAML can be passed in directly as well using `--config` flag! For more details [go.dev reference](https://pkg.go.dev/github.com/bwplotka/mdox) or [Go struct](https://github.com/bwplotka/mdox/blob/main/pkg/transform/config.go).

### Installing

Requirements to build this tool:

* Go 1.15+
* Linux or MacOS

```shell
go get github.com/bwplotka/mdox && go mod tidy
```

or via [bingo](https://github.com/bwplotka/bingo) if want to pin it:

```shell
bingo get -u github.com/bwplotka/mdox
```

## Production Usage

* [Thanos](https://github.com/thanos-io/thanos)
* [Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator)
* [kube-prometheus](https://github.com/prometheus-operator/kube-prometheus)
* [Observatorium](https://github.com/observatorium/observatorium)
* [RedHat Observability Group Handbook](https://github.com/rhobs/handbook)
* [Bingo](https://github.com/bwplotka/bingo)
* [effiecientgo/tools](https://github.com/efficientgo/tools)
* [effiecientgo/e2e](https://github.com/efficientgo/e2e)

## Contributing

Any contributions are welcome! Just use GitHub Issues and Pull Requests as usual. We follow [Thanos Go coding style](https://thanos.io/tip/contributing/coding-style-guide.md/) guide.

Have questions or feedback? Join our [slack channel](https://cloud-native.slack.com/archives/mdox)!

## Initial Author

[@bwplotka](https://bwplotka.dev)

Note: This project was a part of [GSoC'21](https://summerofcode.withgoogle.com/projects/#5053843303301120) (mentor: [@bwplotka](https://bwplotka.dev), mentee: [@saswatamcode](https://saswatamcode.tech)).
