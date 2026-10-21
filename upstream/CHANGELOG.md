# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.3.0] - 2024-08-14

### Changed
- Breaking: the return type of the `Neighbours` method in the `astar.Graph[Node]`
  interface changed from `[]Node` to `iter.Seq[Node]`
- The project now requires Go >= 1.23.0

## [0.2.0] - 2022-03-15

### Changed
- Breaking: the functions now use type parameters (generics)
  for the `Node` type instead of `type Node interface{}`
- The project now requires Go >= 1.18.0

## [0.1.0] - 2021-01-31

### Added
- Initial release

[0.3.0]: https://github.com/fzipp/astar/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/fzipp/astar/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/fzipp/astar/releases/tag/v0.1.0
