---
title: block-registry
---

This repository is the canonical source of the recipes that tell
[block](https://github.com/nao1215/block) how to find and fetch blockchain
development tools. A recipe is a rule, not a list of versions:

```text
upstream publishes a release
        ↓
block discovers it from the repository's tags
        ↓
the recipe says which artifact to take for each platform
```

So a new upstream version needs no change here. A recipe changes only when an
upstream renames its assets, moves repositories, changes how it distributes
builds, or drops a platform — and a scheduled job notices that, not a person.

Every block binary embeds a tested snapshot of these recipes, so
`block list` and `block lock` work offline and a given block version always
pairs with a registry it was tested against.
