package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The point of generating the table is that it says what the recipes say, so
// the test writes recipes and reads the table back.
func TestTableGroupsToolsByEcosystem(t *testing.T) {
	t.Parallel()
	dir := writeRecipes(t, map[string]string{
		"zeta.toml":  recipeTOML("zeta", []string{"cosmos", "ibc"}, "example/zeta"),
		"alpha.toml": recipeTOML("alpha", []string{"cosmos"}, "example/alpha"),
	})
	recipes, err := load(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := table(recipes)

	if !strings.Contains(got, "2 tools across 2 blockchain systems") {
		t.Errorf("the count is wrong:\n%s", got)
	}
	// Sorted by name inside an ecosystem, so the table does not reshuffle when
	// a file is added.
	if !strings.Contains(got, "| cosmos | `alpha`, `zeta` |") {
		t.Errorf("cosmos row is wrong:\n%s", got)
	}
	// A tool serving two systems appears under both.
	if !strings.Contains(got, "| ibc | `zeta` |") {
		t.Errorf("ibc row is wrong:\n%s", got)
	}
}

func TestLoadRejectsARecipeWithNoName(t *testing.T) {
	t.Parallel()
	dir := writeRecipes(t, map[string]string{
		"broken.toml": "ecosystems = [\"cosmos\"]\ndescription = \"x\"\n",
	})
	if _, err := load(dir); err == nil {
		t.Fatal("a recipe without a name was accepted")
	}
}

// replace has to leave everything outside the markers untouched: the README
// around them is written by hand.
func TestReplaceOnlyTouchesTheMarkedBlock(t *testing.T) {
	t.Parallel()
	doc := "before\n\n" + beginMarker + "\n\nold\n\n" + endMarker + "\n\nafter\n"
	got, err := replace(doc, "new\n")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "before\n\n") || !strings.HasSuffix(got, "\n\nafter\n") {
		t.Errorf("text outside the markers changed:\n%s", got)
	}
	if strings.Contains(got, "old") || !strings.Contains(got, "new") {
		t.Errorf("the block was not replaced:\n%s", got)
	}
}

func TestReplaceReportsMissingMarkers(t *testing.T) {
	t.Parallel()
	if _, err := replace("no markers here\n", "new\n"); err == nil {
		t.Fatal("a document without markers was accepted")
	}
	if _, err := replace(endMarker+"\n"+beginMarker+"\n", "new\n"); err == nil {
		t.Fatal("markers in the wrong order were accepted")
	}
}

// The committed README is the artifact this tool exists to keep correct.
func TestCommittedREADMEIsCurrent(t *testing.T) {
	t.Parallel()
	if err := run(filepath.Join("..", "..", "registry"), filepath.Join("..", "..", "README.md"), true); err != nil {
		t.Error(err)
	}
}

func recipeTOML(name string, ecosystems []string, repo string) string {
	quoted := make([]string, 0, len(ecosystems))
	for _, e := range ecosystems {
		quoted = append(quoted, "\""+e+"\"")
	}
	return "name = \"" + name + "\"\n" +
		"ecosystems = [" + strings.Join(quoted, ", ") + "]\n" +
		"description = \"a tool\"\n\n" +
		"[source]\ntype = \"github_release\"\nrepo = \"" + repo + "\"\n" +
		"asset = \"" + name + "_{version}_{os}_{arch}.tar.gz\"\nbin = [\"" + name + "\"]\n"
}

func writeRecipes(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}
