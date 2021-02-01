# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/en/1.0.0/) and this project adheres to [Semantic Versioning](http://semver.org/spec/v2.0.0.html).

NOTE: As semantic versioning states all 0.y.z releases can contain breaking changes in API (flags, grpc API, any backward compatibility)

We use *breaking* word for marking changes that are not backward compatible (relates only to v0.y.z releases.)

## Unreleased

## [v0.2.0](https://github.com/bwplotka/mdox/releases/tag/v0.2.1)

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
