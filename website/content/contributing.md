---
title: Adding a tool
description: "How to add or fix a recipe in the block registry, and what CI checks before it ships."
toc: true
---

## Add a recipe

1. Write `registry/<tool>.toml`. Start from a tool with the same shape — a
   Rust project with target-triple assets, a Go project with `os_arch`
   assets, a vendor download server — and read the
   [recipe format](../recipe/).
2. Run the linter:

   ```shell
   make lint
   ```

3. Prove it works before opening the pull request, using block itself. A
   recipe and a project-local source are the same format, so you can try one
   without touching the registry at all:

   ```toml
   # block.toml, anywhere
   [tools.mytool]
   version = "1"

   [tools.mytool.source]
   type = "github_release"
   repo = "example/mytool"
   asset = "mytool_v{version}_{os}_{arch}.tar.gz"
   bin = ["mytool"]
   ```

   ```shell
   block lock && block sync && block exec mytool --version
   ```

   That is exactly what a user will experience, and it is the fastest way to
   find a wrong asset name or a missing platform.

4. Open a pull request. Say which upstream versions and platforms you tried.

## What CI checks

On every pull request, `registry-lint` validates each recipe offline: the
file name matches `name`, the ecosystems and description are present and
well-formed, the source table has the fields its type needs, placeholders are
ones block knows, executable paths cannot escape the install directory, and
platform names are real. The recipes are also validated against the published
JSON Schema.

On a schedule, block's own repository runs the live check against the
snapshot it embeds: newest stable version per recipe, artifact resolution for
every declared platform, checksum verification, unpacking, and a probe of
every declared executable. That check lives with block because it needs
block's resolver — duplicating resolution here would create a second
definition of the format.

So: this repository owns the *data*, block owns the *behaviour*, and the live
check is what tells a human that an upstream changed.

## When a recipe breaks

Routine upstream releases need no change. A recipe needs a person when the
upstream renames its assets, moves repositories, changes how it distributes
builds, or drops a platform. In each case the fix is a recipe change here,
and it reaches users with the next block release that picks up the snapshot.

An existing `block.lock` is unaffected either way: it records the URL and the
digest it resolved, and `block sync` installs exactly that. Only `block lock`
re-reads recipes.

## Platform coverage

Follow the upstream. If a project ships no macOS x86-64 build, leave it out of
`platforms`: block will say `unsupported platform darwin/amd64` rather than
install something else. Silence about a platform the upstream does not
support is worse than a clear error.
