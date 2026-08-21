# block-registry

Recipes that tell [block](https://github.com/nao1215/block) how to find and
fetch blockchain development tools.

```console
$ block list cosmos
NAME       COMMANDS   DESCRIPTION
cometbft   cometbft   Byzantine fault-tolerant consensus engine and node behind Cosmos SDK chains
gaia       gaiad      Cosmos Hub node (gaiad)
hermes     hermes     IBC relayer connecting Cosmos SDK chains, written in Rust
osmosis    osmosisd   Osmosis appchain node (osmosisd), the Cosmos AMM
```

This repository holds the data behind that listing. Every `block` binary
embeds a tested snapshot of it, so `block list` and `block lock` work offline
and a given block version always pairs with a registry it was tested against.

**Catalog and documentation:** <https://nao1215.github.io/block-registry/>

## A recipe is a rule, not a list of versions

```text
upstream publishes a release
        ↓
block discovers it from the repository's tags
        ↓
the recipe says which artifact to take for each platform
```

A new upstream version needs no change here. A recipe changes only when an
upstream renames its assets, moves repositories, changes how it distributes
builds, or drops a platform.

```toml
# registry/hermes.toml
name = "hermes"
ecosystems = ["cosmos", "ibc"]
description = "IBC relayer connecting Cosmos SDK chains, written in Rust"

[source]
type = "github_release"
repo = "informalsystems/hermes"
asset = "hermes-v{version}-{arch}-{os}.tar.gz"
platforms = ["linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64"]
bin = ["hermes"]

[source.os]
linux = "unknown-linux-gnu"
darwin = "apple-darwin"

[source.arch]
amd64 = "x86_64"
arm64 = "aarch64"
```

The full field reference is in
[the recipe format](https://nao1215.github.io/block-registry/recipe/), and the
machine-readable copy is [schema/recipe.schema.json](./schema/recipe.schema.json).

## What is in here

| Ecosystem | Tools |
| --- | --- |
| bitcoin | `bitcoin-core` |
| ethereum | `foundry`, `solc`, `geth`, `reth`, `lighthouse` |
| solana | `agave`, `anchor`, `surfpool` |
| cosmos | `gaia`, `cometbft`, `osmosis` |
| ibc | `hermes` |

Two install methods cover all of them: `github_release` (a release asset —
archive or single raw executable — using GitHub's own SHA-256 when it records
one) and `http` (a prebuilt artifact on the upstream's own download server).
A third is added only when a blockchain CLI genuinely cannot be obtained with
those, and it will be a type whose meaning and safety boundary block
understands. There is no `install = "curl … | bash"`, and there never will be.

## Adding or fixing a tool

```shell
make lint     # validate every recipe, offline
make test     # the linter's own tests
```

Then try the recipe as a project-local source before opening a pull request —
the format is identical, so `block lock && block sync && block exec <tool>
--version` is exactly what a user will experience. The full walkthrough is in
[Adding a tool](https://nao1215.github.io/block-registry/contributing/).

## How this is checked

| Check | Where | When |
| --- | --- | --- |
| File name, ecosystems, description, source shape, executable-path safety | here (`registry-lint`) | every push and pull request |
| Recipes match the published JSON Schema | here | every push and pull request |
| Newest stable version, artifact per platform, checksum, unpack, `--version` probe | [block](https://github.com/nao1215/block) (`make registry-live`) | weekly, and on demand |

The live check lives with block because it needs block's resolver;
duplicating resolution here would create a second definition of the format.
This repository owns the data, block owns the behaviour, and the scheduled
check is what tells a human that an upstream changed — routine releases never
do.

An existing `block.lock` is unaffected by anything here: it records the URL
and digest it resolved, and `block sync` installs exactly that. Only
`block lock` re-reads recipes.

## Related

- [block](https://github.com/nao1215/block) — the CLI these recipes serve
- [setup-block](https://github.com/nao1215/setup-block) — GitHub Action that installs block and caches its toolchain

## License

[MIT](./LICENSE)
