# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/en/1.0.0/) and this project adheres to [Semantic Versioning](http://semver.org/spec/v2.0.0.html).

NOTE: As semantic versioning states all 0.y.z releases can contain breaking changes in API (flags, grpc API, any backward compatibility)

We use *breaking* word for marking changes that are not backward compatible (relates only to v0.y.z releases.)

## Unreleased

### Added

* [#86](https://github.com/bwplotka/mdox/pull/86) Add configuration options for sending HTTP requests (to help avoid intermittent errors).

### Fixed

* [#84](https://github.com/bwplotka/mdox/pull/84) Allow quotes in first header.

## [v0.9.0](https://github.com/bwplotka/mdox/releases/tag/v0.9.0)

### Added

* [#20](https://github.com/bwplotka/mdox/pull/20) Add quotes to frontmatter
* [#25](https://github.com/bwplotka/mdox/pull/25) Add `sed` testcase for `mdox-exec`
* [#29](https://github.com/bwplotka/mdox/pull/29) Add transform command
* [#33](https://github.com/bwplotka/mdox/pull/33) Add `links.validate.config` flag
* [#40](https://github.com/bwplotka/mdox/pull/40) Add retries to link checking
* [#46](https://github.com/bwplotka/mdox/pull/46) Add support for formatting in-md HTML and comments
* [#47](https://github.com/bwplotka/mdox/pull/47) Add `backMatter` support
* [#48](https://github.com/bwplotka/mdox/pull/48) Add file line numbers to errors
* [#49](https://github.com/bwplotka/mdox/pull/49) Add CLI spinner and colored diff
* [#53](https://github.com/bwplotka/mdox/pull/53) Add profiling and metrics
* [#56](https://github.com/bwplotka/mdox/pull/56) Add benchmarks
* [#58](https://github.com/bwplotka/mdox/pull/58) Add option to preserve header (`popHeader`)
* [#66](https://github.com/bwplotka/mdox/pull/66) Add support for email `mailto:` links
* [#68](https://github.com/bwplotka/mdox/pull/68) Add newline check for `mdox-exec` output
* [#69](https://github.com/bwplotka/mdox/pull/69) Add timeout option to `links.validate.config`
* [#71](https://github.com/bwplotka/mdox/pull/71) Add `Origin.Path` to transform template
* [#75](https://github.com/bwplotka/mdox/pull/75) Add `soft-wraps` flag to preserve soft line breaks
* [#76](https://github.com/bwplotka/mdox/pull/76) Add `linkPrefixForNonMarkdownResources` to transform config
* [#78](https://github.com/bwplotka/mdox/pull/78) Add retry for 0 StatusCode

### Fixed

* [#16](https://github.com/bwplotka/mdox/pull/16) Fixed frontmatter issues
* [#17](https://github.com/bwplotka/mdox/pull/17) Fixed `mdox-exec` issues with --help by adding `mdox-expect-exit-code` code block directive
* [#21](https://github.com/bwplotka/mdox/pull/21) Fixed local anchor link issue
* [#26](https://github.com/bwplotka/mdox/pull/26) Fixed same link in different file error
* [#31](https://github.com/bwplotka/mdox/pull/31) Fixed `clilog` file names for link errors
* [#32](https://github.com/bwplotka/mdox/pull/32) Added support for modifying links in image URLs and fixed linking when adding extra files from outside.
* [#41](https://github.com/bwplotka/mdox/pull/41) Fixed mdox gen errors on command with = inside (common case)
* [#44](https://github.com/bwplotka/mdox/pull/44) Fixed dash header ID
* [#52](https://github.com/bwplotka/mdox/pull/52) Fixed i18n section links
* [#62](https://github.com/bwplotka/mdox/pull/62) Fixed link line number in errors

### Changed

* [#27](https://github.com/bwplotka/mdox/pull/27) Change from `mdox-gen-exec` to `mdox-exec`
* [#39](https://github.com/bwplotka/mdox/pull/39) Change to `efficientgo/tools/extkingpin` for pathOrContent flags
* [#45](https://github.com/bwplotka/mdox/pull/45) Change transform config flag to pathOrContent
* [#59](https://github.com/bwplotka/mdox/pull/59) Make GitHub validator smarter

## [v0.2.1](https://github.com/bwplotka/mdox/releases/tag/v0.2.1)

### Fixed

* Missing dependency issue

## [v0.2.0](https://github.com/bwplotka/mdox/releases/tag/v0.2.0)

### Changed

* Fixed whitespace bug in code blocks.
* Add support for links.
* Changed `--links.localise.address-regex` flag to `--links.localize.address-regex`
* Changed `--links.anchor-dir` flag to `--anchor-dir`.
* Changed `--links.validate.address-regex` flag to `--links.validate.without-address-regex`.
* Allow anchor dir to be relative.
* Improved error formatting.
* By default front matter is getting formatted now.

## [v0.1.0](https://github.com/bwplotka/mdox/releases/tag/v0.1.0)

Initial release.
