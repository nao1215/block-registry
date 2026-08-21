# Contributing to block-registry

Thank you for widening what `block` can install. A good recipe is one nobody
has to think about again: it keeps working as the upstream publishes release
after release, and it fails loudly the day the upstream changes its mind.

## Adding a tool

1. Write `registry/<tool>.toml`. See
   [the recipe format](https://nao1215.github.io/block-registry/recipe/) and
   start from a tool with the same shape.
2. `make lint` — the file name, ecosystems, description, source table,
   placeholders, executable paths and platform names are all checked offline.
3. Try it as a project-local source with block itself, because a recipe and a
   `[tools.<name>.source]` table are the same format:

   ```shell
   block lock && block sync && block exec <tool> --version
   ```

4. Open a pull request saying which versions and platforms you tried.

## What belongs in a recipe

- **Take the artifact from the upstream's own GitHub release** whenever the
  upstream publishes one: block then records the SHA-256 GitHub publishes
  beside it. Only when the upstream builds binaries but does not attach them
  to its releases does a recipe use type `http`, and then the host must be
  added to [policy/hosts.toml](./policy/hosts.toml) in the same change,
  naming the repository it serves and why a release asset will not do.
  `make lint` refuses anything else, and that refusal is the point: widening
  what block installs must never quietly widen where block downloads from.
- **A tool that cannot be had that way does not go in.** Not through a
  package manager, not through an install script, not from a mirror someone
  found. Say so in an issue instead — several upstreams have started
  publishing release assets because someone asked.
- **Follow the upstream's platform coverage.** If there is no macOS x86-64
  build, leave `darwin/amd64` out: a clear "unsupported platform" beats
  installing something else.
- **Say what the tool is, not how good it is.** Descriptions are one plain
  sentence under 100 characters; upstream marketing does not belong here.
- **Never add a way to run arbitrary commands.** No `install`, no `command`,
  no `script`. A recipe is data that block interprets.

## Fixing a broken recipe

The scheduled live check in block's repository is what notices — an asset
renamed, a repository moved, a platform dropped. Fix the recipe here; users
pick it up with the next block release that refreshes the snapshot. Existing
lockfiles keep working either way, because they record the URL and digest they
already resolved.

## Style

- Explain the upstream's quirks in a comment at the top of the recipe: why a
  `target` table is needed, why a platform is missing, which tag line ships
  binaries. The next person to touch it will be looking for exactly that.
- Keep the ecosystem names lower-case and canonical — they are what users type
  after `block list`.
