---
title: Recipe format
description: "Every field of a block registry recipe, and the order of preference between install methods."
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
`{os}`, `{arch}`, `{target}`, and `{commit}` (`http` only — the first 8 hex
digits of the commit the version tag points at).

Archives may be `.tar.gz` / `.tgz`, `.tar.bz2` / `.tbz2` or `.zip`.

`target` exists because some upstreams do not name platforms as a product of
OS and architecture: Bitcoin Core writes `aarch64-linux-gnu` but
`arm64-apple-darwin`. Use `os` and `arch` when they suffice, `target` when
they do not.

## Choosing an install method

A recipe states exactly one method, and block executes it without ever falling
back to another at run time. Pick the highest the upstream really supports:

1. **Official prebuilt GitHub Release artifact** — `github_release`. GitHub
   records a SHA-256 for assets uploaded since 2025, which block writes
   straight into the lockfile without downloading anything.
2. **Official prebuilt artifact on the upstream's own server** — `http`. The
   checksum is recorded on the first download.
3. Official package registry (`go install`, `cargo install`, npm, pipx) — *not
   implemented*.
4. Limited build from official source — *not implemented*.

Every tool in the catalog is served by the first two. A third method will be
added only when a blockchain CLI genuinely cannot be obtained with them, and
it will be a type whose meaning and safety boundary block understands.

There is no `install = "curl … | bash"`, no `command = "make install"`, and no
arbitrary-script escape hatch. A recipe is data.

## What recipes never do

`ecosystems` and `description` are metadata: block uses them to answer *what
can I use for this chain?*, never to choose tools, install them, judge
compatibility between clients, or generate a toolchain. A project's toolchain
is what its own `block.toml` says, and nothing else.
