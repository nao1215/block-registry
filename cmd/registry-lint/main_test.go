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

func TestValidRecipePasses(t *testing.T) {
	t.Parallel()
	problems, err := lintDir(write(t, valid))
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Errorf("a valid recipe was rejected: %v", problems)
	}
}

func TestTheRealRegistryPasses(t *testing.T) {
	t.Parallel()
	problems, err := lintDir(filepath.Join("..", "..", "registry"))
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Errorf("the registry does not lint clean:\n%s", strings.Join(problems, "\n"))
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
			problems, err := lintDir(write(t, tt.body))
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
	problems, err := lintDir(write(t, httpRecipe))
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
			problems, err := lintDir(write(t, tt.body))
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
	if _, err := lintDir(t.TempDir()); err == nil {
		t.Error("an empty directory was accepted")
	}
	if _, err := lintDir(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Error("a missing directory was accepted")
	}
}
