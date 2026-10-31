# `morning` — build-option research

Goal: one CLI, greets you, shows weather + top news, runs on any OS/arch.
Hard constraints from the brief: cross-compiles everywhere, versioned on GitHub
with semver, released by GitHub Actions, tiny docs, shields.io README, maximal copyleft.

---

## The real constraint: how does a binary reach "any OS/architecture"?

Any language can *print* weather. The differentiator is **how many target
binaries one build machine can produce**, and whether the artifact needs a
runtime installed. Four mechanisms exist:

| Mechanism | Meaning | Cost |
|---|---|---|
| **A. Native cross-compile, no C** | one host emits every target directly | needs stdlib with no system deps |
| **B. Cross-compile + C toolchain per target** | linker/sysroot per target | CI matrix × sysroot hell |
| **C. Bundle a runtime** | ship interpreter/JIT with your code | 40–100 MB artifacts |
| **D. Interpreted / source-distributed** | user needs the language installed | fails "compile on any OS/arch" |

Only **A** satisfies the brief cleanly. That is the axis I scored on.

---

## Full survey (35 languages/targets)

Legend: **Cross** = targets from one machine. **Artifact** = what the user downloads.

### Tier 1 — native cross-compile with zero C dependency

| Language | Cross | Artifact | Verdict |
|---|---|---|---|
| **Go** | **47 ports**, one command, CGO off | 5 MB static, no runtime | **Best fit.** Verified below. |
| **Zig** | excellent (`zig build -Dtarget=`) | 1–2 MB static | Cross is world-class; HTTP+TLS+JSON+TUI ecosystem is thin, you'd hand-roll a lot. |
| **Nim** | good (`nim c --os:... --cpu:...`), or via Zig cc | 1–3 MB static | Underrated. Small stdlib, small community, weak TUI libs. |
| **V** | good, built in (`v -os windows`) | small static | Young language, 1.0 still elusive, thin ecosystem. |
| **D** | good with LDC, needs libc per target | 2–5 MB | Solid but CI sysroots annoy; small CLI ecosystem. |
| **Odin** | good, vendored libc | small static | Games/graphics focused, no mature HTTP+TLS stdlib path. |
| **Ada** | GNAT cross toolchains per target | medium | B (per-target toolchain), not A. |
| **Fortran** | cross via GCC toolchains | medium | No sane HTTP/JSON story. |
| **C / C++** | needs per-target musl/mingw | 50 KB–2 MB | Smallest binaries, but you vendor libcurl/OpenSSL/JSON. Weeks of work, permanent maintenance. |

### Tier 2 — native but cross-compile is painful or per-target

| Language | Cross | Verdict |
|---|---|---|
| **Rust** | Needs linker+sysroot per target; `cargo-zigbuild`/`cross` paper over it | Great binaries (3 MB), great TUI (ratatui). Cost: real cross toolchain setup, slower builds, more code for the same app. **Strong #2.** |
| **C# / .NET NativeAOT** | **No cross-OS** — must build on each target OS | Rules out "one machine, any target"; still needs a CI matrix that works. |
| **Swift** | Windows/Linux toolchains exist, cross is unsupported | Toolchain per platform; ecosystem for this is thin. |
| **Haskell** | GHC cross-compiling is a research-grade slog | Disqualified. |
| **OCaml** | cross needs per-target OCaml cross compiler | Disqualified for CI simplicity. |
| **Crystal** | `--cross-compile` only *emits* a compile command; you still need each target's toolchain | Disqualified. |
| **Dart** | `dart compile exe` cannot cross-compile (open issue #28617) | Disqualified. |
| **Julia** | PackageCompiler, no cross | Disqualified. |
| **Chapel / Vala / Haxe** | per-target or transpile | Niche/no win. |

### Tier 3 — runtime bundled (C)
| Language | Cross | Artifact | Verdict |
|---|---|---|---|
| **Deno** | `deno compile --target=` **cross-compiles** | 60–90 MB | Only JS runtime with true cross-compile. Huge binary. |
| **Bun** | `--compile --target=` cross-compiles | 50–90 MB | Same tradeoff, faster than Node. |
| **Node.js SEA** | single-executable, **no cross-compile** | 70 MB+ | Needs one runner per OS. |
| **Elixir/Burrito** | builds per target | 20–40 MB | Cool, niche, heavyweight. |
| **Java (GraalVM native-image)** | **no cross** | 10–20 MB | Matrix-per-OS. |
| **Java/Kotlin (JVM jar)** | jar is portable, but needs a JVM | 200 KB + JRE | Not a standalone binary. |
| **Erlang escript** | needs ERTS | — | No. |
| **Python (PyInstaller/Nuitka)** | **no cross** | 10–30 MB | Startup 200–600 ms; Nuitka+musl is doable but fragile. |
| **Ruby / Perl / PHP / Lua / Tcl / Racket / Clojure(bb) / R / COBOL / Forth / Smalltalk** | source-distributed or per-target bundler | needs interpreter | Fails the compile-everywhere brief. |

### Tier 4 — platform-agnostic bytecode
| Target | Verdict |
|---|---|
| **WASM / WASI** (any language compiling to it) | Genuinely universal, but needs a WASI runtime (`wasmtime`) installed — swaps "install Go" for "install wasmtime". Interesting, not simplest. Go does emit `wasip1/wasm` (verified). |

---

## Empirical proof (run on this Windows box, Go 1.27)

Seven targets from one machine, one `go build` each, no CGO, correct magic bytes:

```
linux/amd64      5,293,905 b   ELF 64-bit LSB
linux/arm64      5,014,079 b   ELF 64-bit
linux/riscv64    4,969,320 b   ELF 64-bit
darwin/arm64     5,097,906 b   Mach-O 64-bit arm64
windows/arm64    4,941,824 b   PE32+ console
freebsd/arm64    4,993,746 b   ELF 64-bit
wasip1/wasm      4,858,487 b   WebAssembly MVP
```

Go's own port list: **47** `GOOS/GOARCH` pairs, all buildable from Windows.

---

## Data sources (all tested keyless — no API keys, no accounts)

| Need | Source | Status |
|---|---|---|
| Weather current + daily | `api.open-meteo.com` | 200, no key, WMO codes, C/F/UOM params |
| City → lat/lon | `geocoding-api.open-meteo.com` | 200, no key |
| Weather fallback | `wttr.in?format=j1` | 200, unofficial, rate-limited |
| Headlines (world) | BBC / NPR / Guardian RSS | 200 |
| Tech headlines | HN Firebase API + hnrss.org + lobste.rs | 200 |

Recommendation: **Open-Meteo + RSS** as primary (stable, no key), HN optional section.
No keys means no secrets in CI and no signup before first run.

---

## Recommended stack

**Go 1.24+ + Lip Gloss (charmbracelet)** — static render, not a full TUI.

- One `go build` per target; CI matrix needs no cross toolchain, no Docker.
- ~5 MB self-contained binary, ~5 ms startup, no runtime.
- Lip Gloss = pure ANSI styling; auto-degrades on dumb terminals; ~200 KB of deps.
- Bubble Tea v2 only if you want arrow-key navigation later (adds an event loop; the
  app is a dashboard, so it's optional).

Runner-up: **Rust + ratatui** (smaller binary, better perf) — pay with cargo-zigbuild
cross setup and ~3× the code. Third: **Zig** (best cross story in existence, thinnest ecosystem).

Release plumbing: **GoReleaser** in GitHub Actions on `v*` tags → matrix of
linux/darwin/windows × amd64/arm64 (+linux arm/riscv64), tar.gz/zip archives,
`checksums.txt`, auto-generated changelog, semver tags.

License: **AGPL-3.0-or-later** — the strongest OSI-recognized copyleft (GPLv3 plus the
§13 network clause). Maximalist alternatives: RPL-1.5 / OSL-3.0 (arguably stricter, far
less recognized, worse tooling). SSPL is not OSI-approved. For a CLI, AGPL ≈ GPL in
practice but leaves no network loophole if a server mode is ever added.
