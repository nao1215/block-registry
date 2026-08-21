# Security policy

## What this repository is

block-registry holds data, not code that runs on your machine. A recipe is a
TOML file that tells [block](https://github.com/nao1215/block) which upstream
repository a tool comes from and which artifact to take for each platform.
There is no `install = "curl ... | bash"`, no `command = ...`, and no
arbitrary-script escape hatch: adding a tool here cannot add a way to run
something.

The consequence is that the security question here is about provenance —
"does this recipe point where it claims to point?" — and the behaviour
question is in block, which does the downloading, the checksum verification
and the extraction.

## Reporting a vulnerability

Report security issues privately, not through public issues or pull requests.

- Email: n.chika156@gmail.com
- Or use the "Report a vulnerability" button on the repository's Security tab.

Reports in these areas are especially valuable:

- A recipe whose `repo`, `asset` or `url` resolves to an artifact its upstream
  did not publish — a typosquatted repository, a fork wearing an official
  name, a download host that is not the project's own.
- A `url` that reaches a host `policy/hosts.toml` does not list for that
  repository, or a `policy/hosts.toml` entry that names a host the upstream
  does not actually control.
- A recipe whose `bin` paths could write outside the install directory once
  extracted (absolute paths, `..` components).
- A way to make `registry-lint` pass a recipe that breaks any of the above.

Include the recipe file and what you expected the resolved artifact to be.

## What is already enforced

`registry-lint` runs on every push and pull request and refuses:

- a recipe whose file name does not match its `name`;
- a `github_release` recipe pointing at a repository other than the one its
  version tags come from;
- an `http` recipe whose url reaches any host not listed for that repository
  in [policy/hosts.toml](./policy/hosts.toml), including a placeholder host;
- a `github.com` url wearing type `http`, which would throw away the SHA-256
  GitHub publishes beside the asset;
- an executable path that is absolute or escapes the install directory.

A `policy/hosts.toml` entry that no recipe uses is reported too, so the set of
hosts block downloads from shrinks as well as grows.

## What block enforces at install time

Anything a recipe resolves to is still verified by block before it runs:
HTTPS only with no downgrade on redirect, SHA-256 against `block.lock` before
extraction, and defensive extraction that refuses absolute paths, `..`
components, symlinks and hard links. See
[block's SECURITY.md](https://github.com/nao1215/block/blob/main/SECURITY.md).

An existing `block.lock` is unaffected by anything merged here: it records the
URL and digest it resolved, and `block sync` installs exactly that.
