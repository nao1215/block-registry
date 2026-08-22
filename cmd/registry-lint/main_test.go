package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// valid is the recipe every case below starts from: a real shape, so that a
// test failing means the rule under test, not the fixture, is wrong.
const valid = `name = "tool"
ecosystems = ["cosmos", "ibc"]
description = "A tool that does one thing"

[source]
type = "github_release"
repo = "owner/tool"
asset = "tool_v{version}_{os}_{arch}.tar.gz"
platforms = ["linux/amd64", "darwin/arm64"]
bin = ["bin/tool"]
strip_components = 1

[source.os]
darwin = "apple-darwin"
`

func write(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tool.toml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

// testPolicy is the allowlist the fixtures below download against: one vendor
// host for one repository, the same shape the real policy has.
func testPolicy(t *testing.T) *policy {
	t.Helper()
	return writePolicy(t, `[[host]]
host = "example.org"
repo = "owner/tool"
why = "The fixture upstream publishes binaries on its own site."
`)
}

// realPolicy is this repository's own policy, so that the recipes are linted
// against the file that actually ships.
func realPolicy(t *testing.T) *policy {
	t.Helper()
	pol, err := loadPolicy(filepath.Join("..", "..", "policy", "hosts.toml"))
	if err != nil {
		t.Fatal(err)
	}
	return pol
}

func writePolicy(t *testing.T, body string) *policy {
	t.Helper()
	path := filepath.Join(t.TempDir(), "hosts.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	pol, err := loadPolicy(path)
	if err != nil {
		t.Fatal(err)
	}
	return pol
}

func TestValidRecipePasses(t *testing.T) {
	t.Parallel()
	problems, err := lintDir(write(t, valid), testPolicy(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Errorf("a valid recipe was rejected: %v", problems)
	}
}

func TestTheRealRegistryPasses(t *testing.T) {
	t.Parallel()
	pol := realPolicy(t)
	problems, err := lintDir(filepath.Join("..", "..", "registry"), pol)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Errorf("the registry does not lint clean:\n%s", strings.Join(problems, "\n"))
	}
	if stale := pol.unused(); len(stale) != 0 {
		t.Errorf("the download-source policy has entries no recipe needs:\n%s", strings.Join(stale, "\n"))
	}
}

func TestProblemsAreCaught(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, body, want string
	}{
		{"name does not match the file", strings.Replace(valid, `name = "tool"`, `name = "other"`, 1), `name is "other" but the file is tool.toml`},
		{"upper-case name", strings.Replace(valid, `name = "tool"`, `name = "Tool"`, 1), "invalid name"},
		// A stray top-level key: appending would land inside [source.os].
		{"unknown key", strings.Replace(valid, `name = "tool"`, "name = \"tool\"\nhomepage = \"x\"", 1), `unknown key "homepage"`},
		{"no ecosystems", strings.Replace(valid, `ecosystems = ["cosmos", "ibc"]`, `ecosystems = []`, 1), "ecosystems is required"},
		{"duplicate ecosystem", strings.Replace(valid, `["cosmos", "ibc"]`, `["ibc", "ibc"]`, 1), `ecosystem "ibc" is listed twice`},
		{"upper-case ecosystem", strings.Replace(valid, `["cosmos", "ibc"]`, `["Cosmos"]`, 1), "invalid name"},
		{"no description", strings.Replace(valid, `description = "A tool that does one thing"`, `description = ""`, 1), "description is required"},
		{"padded description", strings.Replace(valid, `"A tool that does one thing"`, `" A tool "`, 1), "leading or trailing whitespace"},
		{"long description", strings.Replace(valid, `"A tool that does one thing"`, `"`+strings.Repeat("x", maxDescription+1)+`"`, 1), "keep it under 100"},
		{"unknown source type", strings.Replace(valid, `type = "github_release"`, `type = "cargo"`, 1), "unsupported source type"},
		{"url on a release source", strings.Replace(valid, `repo = "owner/tool"`, "repo = \"owner/tool\"\nurl = \"https://example.org/x-{version}.tar.gz\"", 1), `url is only valid for type "http"`},
		{"bad repo", strings.Replace(valid, `repo = "owner/tool"`, `repo = "tool"`, 1), "invalid repo"},
		{"unknown placeholder", strings.Replace(valid, "{os}_{arch}", "{os}_{platform}", 1), "unknown placeholder {platform}"},
		{"target without placeholder", valid + "\n[source.target]\n\"linux/amd64\" = \"x86_64-linux\"\n", "{target} is never used"},
		{"empty bin", strings.Replace(valid, `bin = ["bin/tool"]`, `bin = []`, 1), "bin must list at least one"},
		{"duplicate bin", strings.Replace(valid, `bin = ["bin/tool"]`, `bin = ["bin/tool", "bin/tool"]`, 1), "is listed twice"},
		{"escaping bin", strings.Replace(valid, `bin = ["bin/tool"]`, `bin = ["../tool"]`, 1), "invalid bin entry"},
		{"absolute bin", strings.Replace(valid, `bin = ["bin/tool"]`, `bin = ["/usr/bin/tool"]`, 1), "invalid bin entry"},
		{"unsupported platform", strings.Replace(valid, `"darwin/arm64"`, `"plan9/amd64"`, 1), `unsupported platform "plan9/amd64"`},
		{"negative strip", strings.Replace(valid, "strip_components = 1", "strip_components = -1", 1), "must not be negative"},
		{"strip on a raw executable", strings.Replace(valid, `asset = "tool_v{version}_{os}_{arch}.tar.gz"`, `asset = "tool-{os}"`, 1), "strip_components is only valid for archives"},
		{"nested bin for a raw executable", strings.Replace(strings.Replace(valid, `asset = "tool_v{version}_{os}_{arch}.tar.gz"`, `asset = "tool-{os}"`, 1), "strip_components = 1", "", 1), "needs exactly one bare bin name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			problems, err := lintDir(write(t, tt.body), testPolicy(t))
			if err != nil {
				t.Fatal(err)
			}
			if len(problems) == 0 {
				t.Fatalf("no problem reported, want one containing %q", tt.want)
			}
			joined := strings.Join(problems, "\n")
			if !strings.Contains(joined, tt.want) {
				t.Errorf("problems =\n%s\nwant one containing %q", joined, tt.want)
			}
			if !strings.HasPrefix(problems[0], "tool.toml: ") {
				t.Errorf("a problem should name its file: %q", problems[0])
			}
		})
	}
}

func TestHTTPSource(t *testing.T) {
	t.Parallel()
	const httpRecipe = `name = "tool"
ecosystems = ["bitcoin"]
description = "A tool from a vendor download server"

[source]
type = "http"
repo = "owner/tool"
url = "https://example.org/tool-{version}-{target}.tar.gz"
strip_components = 1
bin = ["bin/tool"]

[source.target]
"linux/amd64" = "x86_64-linux-gnu"
"darwin/arm64" = "arm64-apple-darwin"
`
	problems, err := lintDir(write(t, httpRecipe), testPolicy(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Errorf("a valid http recipe was rejected: %v", problems)
	}

	for _, tt := range []struct{ name, body, want string }{
		{"plain http", strings.Replace(httpRecipe, "https://", "http://", 1), "must start with https://"},
		{"no version", strings.Replace(httpRecipe, "tool-{version}-{target}", "tool-{target}", 1), "must contain {version}"},
		{"asset on an http source", strings.Replace(httpRecipe, `bin = ["bin/tool"]`, `asset = "x.tar.gz"`+"\n"+`bin = ["bin/tool"]`, 1), `asset is only valid for type "github_release"`},
		{"target missing", strings.Replace(httpRecipe, "{target}", "{os}", 1), "{target} is never used"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			problems, err := lintDir(write(t, tt.body), testPolicy(t))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(strings.Join(problems, "\n"), tt.want) {
				t.Errorf("problems = %v, want one containing %q", problems, tt.want)
			}
		})
	}
}

func TestEmptyDirectoryIsAnError(t *testing.T) {
	t.Parallel()
	if _, err := lintDir(t.TempDir(), testPolicy(t)); err == nil {
		t.Error("an empty directory was accepted")
	}
	if _, err := lintDir(filepath.Join(t.TempDir(), "missing"), testPolicy(t)); err == nil {
		t.Error("a missing directory was accepted")
	}
}

// The download-source rule: a recipe reaches a GitHub release of the
// repository it names, or a host the policy allows for that repository.
// Nothing else, whatever the URL looks like.
func TestDownloadSourcePolicy(t *testing.T) {
	t.Parallel()
	const vendor = `name = "tool"
ecosystems = ["bitcoin"]
description = "A tool from a vendor download server"

[source]
type = "http"
repo = "owner/tool"
url = "https://example.org/tool-{version}-{os}.tar.gz"
bin = ["tool"]
`
	for _, tt := range []struct{ name, body, want string }{
		{
			"an unlisted host",
			strings.Replace(vendor, "example.org", "downloads.example.net", 1),
			`url host "downloads.example.net" is not allowed for owner/tool`,
		},
		{
			// The host is allowed, but for someone else's repository.
			"the right host for the wrong repository",
			strings.Replace(vendor, `repo = "owner/tool"`, `repo = "someone/else"`, 1),
			`url host "example.org" is not allowed for someone/else`,
		},
		{
			"a github release reached by url",
			strings.Replace(vendor, "https://example.org/tool-{version}-{os}.tar.gz",
				"https://github.com/owner/tool/releases/download/v{version}/tool-{os}.tar.gz", 1),
			`use type "github_release"`,
		},
		{
			// A templated host would mean the allowlist could not say what a
			// recipe downloads from, which is the whole point of having one.
			"a templated host",
			strings.Replace(vendor, "https://example.org/", "https://{os}.example.org/", 1),
			"must be a constant",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			problems, err := lintDir(write(t, tt.body), testPolicy(t))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(strings.Join(problems, "\n"), tt.want) {
				t.Errorf("problems = %v, want one containing %q", problems, tt.want)
			}
		})
	}

	t.Run("an allowed host passes and is reported as used", func(t *testing.T) {
		t.Parallel()
		pol := testPolicy(t)
		problems, err := lintDir(write(t, vendor), pol)
		if err != nil {
			t.Fatal(err)
		}
		if len(problems) != 0 {
			t.Errorf("an allowed vendor host was rejected: %v", problems)
		}
		if !strings.Contains(pol.summary, "1 from an allowed vendor host") {
			t.Errorf("summary = %q", pol.summary)
		}
	})

	// An entry nothing downloads from is a problem too: the allowlist must
	// shrink when a tool moves back to GitHub releases.
	t.Run("a stale policy entry", func(t *testing.T) {
		t.Parallel()
		pol := testPolicy(t)
		if _, err := lintDir(write(t, valid), pol); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(strings.Join(pol.unused(), "\n"), "no recipe downloads from it") {
			t.Errorf("unused() = %v, want the host reported", pol.unused())
		}
	})
}

func TestPolicyFileIsItselfChecked(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ name, body, want string }{
		{"no host", "[[host]]\nrepo = \"o/r\"\nwhy = \"because\"\n", "has no host"},
		{"no repo", "[[host]]\nhost = \"example.org\"\nwhy = \"because\"\n", "which repository"},
		{"no reason", "[[host]]\nhost = \"example.org\"\nrepo = \"o/r\"\n", "why a GitHub release asset will not do"},
		{"github", "[[host]]\nhost = \"github.com\"\nrepo = \"o/r\"\nwhy = \"because\"\n", `must use type "github_release"`},
		{"unknown key", "[[host]]\nhost = \"example.org\"\nrepo = \"o/r\"\nwhy = \"because\"\nnote = \"x\"\n", `unknown key`},
		{
			"listed twice",
			"[[host]]\nhost = \"example.org\"\nrepo = \"o/r\"\nwhy = \"because\"\n[[host]]\nhost = \"example.org\"\nrepo = \"o/r\"\nwhy = \"again\"\n",
			"listed twice",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "hosts.toml")
			if err := os.WriteFile(path, []byte(tt.body), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := loadPolicy(path)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("loadPolicy() error = %v, want one containing %q", err, tt.want)
			}
		})
	}
	if _, err := loadPolicy(filepath.Join(t.TempDir(), "missing.toml")); err == nil {
		t.Error("a missing policy file was accepted")
	}
}

// A channel is a release line published under a tag that moves. What a recipe
// declares about one is how that release names its assets, and the linter
// holds it to the same rules the versioned template has.
func TestChannels(t *testing.T) {
	t.Parallel()
	withChannel := func(body string) string {
		return valid + "\n[source.channels.nightly]\n" + body
	}
	problems, err := lintDir(write(t, withChannel(`asset = "tool_nightly_{os}_{arch}.tar.gz"`+"\n")), testPolicy(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Errorf("a well-formed channel was refused: %v", problems)
	}
	tests := []struct{ name, body, want string }{
		{"no asset", `asset = ""` + "\n", "asset template is required"},
		{"a path", `asset = "dir/tool_nightly.tar.gz"` + "\n", "must be a bare file name"},
		{"a version", `asset = "tool_{version}_{os}.tar.gz"` + "\n", "a channel release has no version"},
		{"another kind of artifact", `asset = "tool_nightly_{os}"` + "\n", "not the same kind of artifact"},
		{"an unknown placeholder", `asset = "tool_nightly_{channel}.tar.gz"` + "\n", "unknown placeholder {channel}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			problems, err := lintDir(write(t, withChannel(tt.body)), testPolicy(t))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(strings.Join(problems, "\n"), tt.want) {
				t.Errorf("problems = %v, want one containing %q", problems, tt.want)
			}
		})
	}
	// A channel name is spelled the way a constraint is.
	problems, err = lintDir(write(t, valid+"\n[source.channels.Nightly]\nasset = \"tool_nightly_{os}_{arch}.tar.gz\"\n"), testPolicy(t))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(problems, "\n"), "lower-case letters") {
		t.Errorf("an upper-case channel name was accepted: %v", problems)
	}
}
