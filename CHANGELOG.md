# Changelog

All notable changes to this project are documented here.
This project follows [Semantic Versioning](https://semver.org) and
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

## [1.3.2] - 2026-10-31

### Fixed

- The weather glyph was separated from its description by a single space, which
  terminal fonts draw as touching (`☂Drizzle`). The glyph is now padded to a
  three-cell column, so it reads `☂  Drizzle` in any font.

## [1.3.1] - 2026-10-31

### Fixed

- Zones that tzdata names with a bare offset printed as a stray number beside
  the clock (`23:32 +06` for Dhaka, `+0545` for Kathmandu). They now read
  `UTC+6` and `UTC+5:45`.
- Weather glyphs are now all BMP characters with text presentation, so every
  terminal draws them exactly one cell wide. The emoji set (🌦 🌧 🌨 🌫 🌤) is two
  cells in some terminals and one in others, which made the glyph bleed into the
  description on Windows Terminal.

## [1.3.0] - 2026-10-31

### Added

- **Clickable headlines.** Every headline and on-this-day entry is wrapped in an
  OSC 8 terminal hyperlink, on all wrapped lines, so it opens in the browser
  with the terminal's usual click or open-link action. Enabled automatically on
  a terminal; `-links auto|always|never` controls it.
- The layout is now chosen by "rich vs plain" rather than by colour, so
  `-color never` still produces the dashboard with working links.

### Changed

- `-plain` and piped output never carry link or colour escapes.

## [1.2.0] - 2026-10-31

### Added

- **Local headlines.** A `local` section searches Google News RSS (Bing News as
  fallback) for the reported city and tags each item with its outlet.
- **Local history.** "On this day" now prefers events within 50 km of the city,
  heading itself `ON THIS DAY IN <CITY>` when it finds any. Matching requires
  both Wikidata coordinates and the city's name in the article or text, so a
  broadly-geolocated entity cannot pass as local.
- `-sections local` (aliases `city`, `citynews`); `local` joins the defaults.
- `local` and `on_this_day_city` fields in `-json` output.

### Changed

- The place is now resolved once and shared by weather, local news and local
  history, instead of being tied to the weather fetch.
- RSS parsing prefers an item's own `<source>` outlet and strips the duplicated
  `- Outlet` suffix from search-feed titles.

## [1.1.0] - 2026-10-31

### Added

- The clock, the greeting and the temperature unit now follow the reported
  city. `-city Tokyo` prints JST and °C; `-city Boston` prints EDT and °F. When
  the city's zone differs from your machine's, your own time is shown in
  brackets.
- `-units auto` (the new default) picks °F for the countries that use it and °C
  everywhere else. `-units c` and `-units f` still override it.
- `units` field in `-json` output.
- The IANA zone database is embedded (`time/tzdata`), so city timezones resolve
  on hosts with no zoneinfo installed.

### Changed

- `-units` defaults to `auto` instead of `c`, and the config's `"units"` accepts
  `"auto"`.

## [1.0.1] - 2026-10-31

### Fixed

- `go install github.com/epicshawty/morning@latest` reported `morning dev`.
  The release version now falls back to the module build info, so installed
  binaries report their real tag. Local builds still report `dev`.

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

[Unreleased]: https://github.com/epicshawty/morning/compare/v1.3.2...HEAD
[1.3.2]: https://github.com/epicshawty/morning/releases/tag/v1.3.2
[1.3.1]: https://github.com/epicshawty/morning/releases/tag/v1.3.1
[1.3.0]: https://github.com/epicshawty/morning/releases/tag/v1.3.0
[1.2.0]: https://github.com/epicshawty/morning/releases/tag/v1.2.0
[1.1.0]: https://github.com/epicshawty/morning/releases/tag/v1.1.0
[1.0.1]: https://github.com/epicshawty/morning/releases/tag/v1.0.1
[1.0.0]: https://github.com/epicshawty/morning/releases/tag/v1.0.0
