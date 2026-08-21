---
title: Recipe format
description: "Every field of a block registry recipe, and the rule that decides where it may download from."
toc: true
---

One TOML file per tool, named after the tool. The
[JSON Schema](../schema/recipe.schema.json) is the machine-readable copy of
what follows; point your editor at it while writing one.

```toml
name = "hermes"                         # must equal the file name
ecosystems = ["cosmos", "ibc"]          # the blockchain systems it serves
description = "IBC relayer connecting Cosmos SDK chains, written in Rust"

[source]
type = "github_release"                 # or "http"
repo = "informalsystems/hermes"         # versions come from this repo's tags
# tag_prefix = "v"                      # text before MAJOR.MINOR[.PATCH]
asset = "hermes-v{version}-{arch}-{os}.tar.gz"
platforms = ["linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64"]
bin = ["hermes"]                        # executables, relative to the archive root

[source.os]                             # rename {os} (Go's GOOS)
linux = "unknown-linux-gnu"
darwin = "apple-darwin"

[source.arch]                           # rename {arch} (Go's GOARCH)
amd64 = "x86_64"
arm64 = "aarch64"
```

## Fields

| Field | Applies to | Meaning |
| --- | --- | --- |
| `name` | both | The tool's name. Must equal the file name. |
| `ecosystems` | both | Blockchain systems the tool serves. At least one. Discovery and display only. |
| `description` | both | One plain sentence, under 100 characters, no line breaks. |
| `source.type` | both | `github_release` or `http`. |
| `source.repo` | both | The `owner/name` repository whose tags are the version source. |
| `source.tag_prefix` | both | Text before the version in a tag. Defaults to `v`; use `""` for bare tags. |
| `source.asset` | `github_release` | Asset file name template. Without an archive extension it is a single raw executable. |
| `source.url` | `http` | HTTPS URL template of the artifact. |
| `source.strip_components` | both | Leading path components to drop when unpacking. |
| `source.bin` | both | Executables inside the artifact. For a raw executable, the one name to install it under. |
| `source.platforms` | both | The `os/arch` pairs the upstream ships. Empty means all four, or the keys of `target`. |
| `source.os`, `source.arch` | both | Rename `{os}` / `{arch}` for this upstream. |
| `source.target` | both | Maps a whole `os/arch` pair to the upstream's platform string, for `{target}`. |

Placeholders: `{version}` (as the upstream spells it, without the tag prefix),
`{os}`, `{arch}`, `{target}`, and `{commit}` — the first 8 hex digits of the
commit the version tag points at, for the upstreams that stamp the build
commit into the artifact's name (vyper, Nimbus, go-ethereum).

Platforms are `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`,
`windows/amd64` and `windows/arm64`. List only the ones the upstream really
ships: block reports an unsupported platform rather than substituting
another, which is the honest answer and the useful one.

Archives may be `.tar.gz` / `.tgz`, `.tar.bz2` / `.tbz2` or `.zip`.

`target` exists because some upstreams do not name platforms as a product of
OS and architecture: Bitcoin Core writes `aarch64-linux-gnu` but
`arm64-apple-darwin`. Use `os` and `arch` when they suffice, `target` when
they do not.

## Where a recipe may download from

A recipe states exactly one download source, and block executes it without
ever falling back to another at run time. Which source is allowed is not a
matter of taste — it is a rule the linter enforces on every push, so that a
catalog which keeps growing cannot quietly grow the set of places block
fetches binaries from.

1. A GitHub Release asset of the repository the recipe already names —
   `github_release`. The artifact and the version tag come from the same
   project, and GitHub records a SHA-256 for assets uploaded since 2025,
   which block writes straight into the lockfile without downloading
   anything. Prefer this whenever the upstream publishes one.
2. A prebuilt artifact on the upstream's own download server — `http`,
   and only from a host listed in
   [`policy/hosts.toml`](https://github.com/nao1215/block-registry/blob/main/policy/hosts.toml).
   Each entry names the one repository the host serves and why a release
   asset will not do. The checksum is recorded on the first download.

`registry-lint` refuses a url whose host is not listed for that repository, a
url whose host is itself a placeholder, and a `github.com` url wearing type
`http` when `github_release` would carry the published digest. It also
reports an allowlist entry no recipe uses, so the list shrinks when an
upstream starts attaching binaries to its releases.

A host qualifies for tier 2 only if the upstream project operates it or names
it as its release location in the project's own documentation. Third-party
mirrors, package-manager CDNs, personal forks and file-sharing services do
not, however convenient they are.

There is no tier 3. No `install = "curl … | bash"`, no `command = "make
install"`, no package-manager shell-out, and no arbitrary-script escape
hatch. A recipe is data. block does not manage language runtimes either, so a
tool distributed only through npm, PyPI or crates.io is not in the catalog —
and saying so in an upstream issue has, more than once, produced release
assets.

## What recipes never do

`ecosystems` and `description` are metadata: block uses them to answer *what
can I use for this chain?*, never to choose tools, install them, judge
compatibility between clients, or generate a toolchain. A project's toolchain
is what its own `block.toml` says, and nothing else.
