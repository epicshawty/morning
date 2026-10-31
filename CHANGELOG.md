# Changelog

All notable changes to this project are documented here.
This project follows [Semantic Versioning](https://semver.org) and
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

## [1.0.0] - 2026-10-31

### Added

- `morning` prints a one-screen briefing: greeting, weather, world news, tech
  headlines, and an "on this day" fact.
- Weather from Open-Meteo with current conditions, high/low, rain chance, wind,
  sunrise and sunset; location auto-detected by IP or set with `-city`.
- World headlines merged from BBC, NPR, Guardian and Al Jazeera RSS feeds.
- Tech headlines from the Hacker News API and Lobsters, interleaved.
- Styled terminal output via Lip Gloss, with automatic plain-text output when
  stdout is not a TTY. Honours `NO_COLOR`.
- `-json` output for scripting.
- Ten-minute on-disk response cache (`-no-cache` to bypass).
- Section failures degrade to a notice instead of aborting the run.
- Static binaries for Linux, macOS, Windows, FreeBSD, OpenBSD, NetBSD, Solaris,
  illumos and AIX across amd64, arm64, arm, 386, riscv64, ppc64le, s390x,
  mips64le and loong64.

[Unreleased]: https://github.com/epicshawty/morning/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/epicshawty/morning/releases/tag/v1.0.0
