// Command registry-lint checks every recipe in this repository before it can
// reach a block release.
//
// It validates what block itself cannot see — the file name, the ecosystems
// and the description — and the shape of the source table, offline and in
// under a second. What a recipe *resolves to* is not checked here: that needs
// block's resolver, and duplicating it would create a second definition of
// the format. block runs those live checks against the snapshot it embeds
// (see the README).
//
//	registry-lint            # lint ./registry
//	registry-lint ../other   # lint another directory
package main

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// maxDescription keeps a description short enough to sit in a terminal
// column next to the tool's name.
const maxDescription = 100

// archiveExtensions are the archive formats block can unpack. An asset name
// with none of them is a single raw executable.
var archiveExtensions = []string{".tar.gz", ".tgz", ".tar.bz2", ".tbz2", ".zip"} //nolint:gochecknoglobals // immutable table

// platforms are the os/arch pairs block installs for.
var platforms = map[string]bool{ //nolint:gochecknoglobals // immutable table
	"linux/amd64": true, "linux/arm64": true,
	"darwin/amd64": true, "darwin/arm64": true,
}

// recipe mirrors the file format. Unknown keys are an error, so a typo in a
// field name cannot be silently ignored.
type recipe struct {
	Name        string `toml:"name"`
	Ecosystems  []string
	Description string
	Source      source
}

type source struct {
	Type            string
	Repo            string
	TagPrefix       *string `toml:"tag_prefix"`
	Asset           string
	URL             string `toml:"url"`
	StripComponents int    `toml:"strip_components"`
	Platforms       []string
	Bin             []string
	OS              map[string]string `toml:"os"`
	Arch            map[string]string
	Target          map[string]string
}

func main() {
	dir := "registry"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	problems, err := lintDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "registry-lint: %v\n", err)
		os.Exit(2) //nolint:mnd // 2: could not run, as distinct from "found problems"
	}
	for _, p := range problems {
		fmt.Fprintln(os.Stderr, p)
	}
	if len(problems) > 0 {
		fmt.Fprintf(os.Stderr, "\n%d problem(s) in %s\n", len(problems), dir)
		os.Exit(1)
	}
	fmt.Printf("registry-lint: every recipe in %s is valid\n", dir)
}

// lintDir returns one message per problem, in file order.
func lintDir(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".toml") {
			files = append(files, e.Name())
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no recipes found in %s", dir)
	}
	sort.Strings(files)
	var problems []string
	for _, name := range files {
		for _, err := range lintFile(filepath.Join(dir, name)) {
			problems = append(problems, fmt.Sprintf("%s: %v", name, err))
		}
	}
	return problems, nil
}

func lintFile(path string) []error {
	data, err := os.ReadFile(path) //nolint:gosec // linting the repository's own files
	if err != nil {
		return []error{err}
	}
	var r recipe
	md, err := toml.Decode(string(data), &r)
	if err != nil {
		return []error{err}
	}
	var errs []error
	for _, key := range md.Undecoded() {
		errs = append(errs, fmt.Errorf("unknown key %q", key.String()))
	}
	errs = append(errs, r.lint(filepath.Base(path))...)
	return errs
}

func (r recipe) lint(fileName string) []error {
	var errs []error
	want := strings.TrimSuffix(fileName, ".toml")
	if r.Name != want {
		errs = append(errs, fmt.Errorf("name is %q but the file is %s", r.Name, fileName))
	}
	if err := validName(r.Name); err != nil {
		errs = append(errs, err)
	}
	errs = append(errs, r.lintEcosystems()...)
	errs = append(errs, r.lintDescription()...)
	errs = append(errs, r.Source.lint()...)
	return errs
}

func (r recipe) lintEcosystems() []error {
	if len(r.Ecosystems) == 0 {
		return []error{errors.New("ecosystems is required: list the blockchain systems the tool serves")}
	}
	var errs []error
	seen := map[string]bool{}
	for _, e := range r.Ecosystems {
		if err := validName(e); err != nil {
			errs = append(errs, fmt.Errorf("ecosystem: %w", err))
		}
		if seen[e] {
			errs = append(errs, fmt.Errorf("ecosystem %q is listed twice", e))
		}
		seen[e] = true
	}
	return errs
}

func (r recipe) lintDescription() []error {
	switch d := r.Description; {
	case strings.TrimSpace(d) == "":
		return []error{errors.New("description is required: one sentence saying what the tool is")}
	case d != strings.TrimSpace(d):
		return []error{fmt.Errorf("description %q has leading or trailing whitespace", d)}
	case strings.ContainsAny(d, "\n\r\t"):
		return []error{errors.New("description must be a single line")}
	case len(d) > maxDescription:
		return []error{fmt.Errorf("description is %d characters long, keep it under %d", len(d), maxDescription)}
	}
	return nil
}

func (s source) lint() []error {
	var errs []error
	switch s.Type {
	case "github_release":
		if s.URL != "" {
			errs = append(errs, errors.New(`url is only valid for type "http"`))
		}
		errs = append(errs, s.lintAsset()...)
	case "http":
		if s.Asset != "" {
			errs = append(errs, errors.New(`asset is only valid for type "github_release"`))
		}
		errs = append(errs, s.lintURL()...)
	default:
		errs = append(errs, fmt.Errorf("unsupported source type %q: want \"github_release\" or \"http\"", s.Type))
	}
	if owner, name, ok := strings.Cut(s.Repo, "/"); !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		errs = append(errs, fmt.Errorf("invalid repo %q: want owner/name", s.Repo))
	}
	if s.StripComponents < 0 {
		errs = append(errs, errors.New("strip_components must not be negative"))
	}
	errs = append(errs, s.lintBin()...)
	errs = append(errs, s.lintPlatforms()...)
	return errs
}

func (s source) lintAsset() []error {
	var errs []error
	if s.Asset == "" {
		return []error{errors.New("asset template is required")}
	}
	if strings.ContainsAny(s.Asset, "/\\") {
		errs = append(errs, fmt.Errorf("asset template %q must be a bare file name", s.Asset))
	}
	if strings.Contains(s.Asset, "{commit}") {
		errs = append(errs, errors.New("{commit} is only valid in an http url"))
	}
	return append(errs, s.lintTemplate(s.Asset)...)
}

func (s source) lintURL() []error {
	if s.URL == "" {
		return []error{errors.New("url template is required")}
	}
	var errs []error
	if !strings.HasPrefix(s.URL, "https://") {
		errs = append(errs, fmt.Errorf("url template %q must start with https://", s.URL))
	}
	if !strings.Contains(s.URL, "{version}") {
		errs = append(errs, fmt.Errorf("url template %q must contain {version}", s.URL))
	}
	return append(errs, s.lintTemplate(s.URL)...)
}

// lintTemplate checks the placeholders a template uses and the tables they
// need, and that an archive extension block can unpack was chosen.
func (s source) lintTemplate(tmpl string) []error {
	var errs []error
	for _, ph := range placeholders(tmpl) {
		switch ph {
		case "version", "os", "arch", "commit", "target":
		default:
			errs = append(errs, fmt.Errorf("unknown placeholder {%s}", ph))
		}
	}
	if strings.Contains(tmpl, "{target}") && len(s.Target) == 0 {
		errs = append(errs, errors.New("{target} needs a [source.target] table"))
	}
	if len(s.Target) > 0 && !strings.Contains(tmpl, "{target}") {
		errs = append(errs, errors.New("[source.target] is defined but {target} is never used"))
	}
	if isArchive(tmpl) && s.StripComponents > 0 {
		return errs
	}
	if !isArchive(tmpl) {
		if s.StripComponents != 0 {
			errs = append(errs, errors.New("strip_components is only valid for archives"))
		}
		if len(s.Bin) != 1 || strings.Contains(firstOr(s.Bin, ""), "/") {
			errs = append(errs, fmt.Errorf("a raw executable %q needs exactly one bare bin name", tmpl))
		}
	}
	return errs
}

func (s source) lintBin() []error {
	if len(s.Bin) == 0 {
		return []error{errors.New("bin must list at least one executable")}
	}
	var errs []error
	seen := map[string]bool{}
	for _, b := range s.Bin {
		if err := validBin(b); err != nil {
			errs = append(errs, err)
		}
		if seen[b] {
			errs = append(errs, fmt.Errorf("bin %q is listed twice", b))
		}
		seen[b] = true
	}
	return errs
}

func (s source) lintPlatforms() []error {
	var errs []error
	check := func(field string, names []string) {
		seen := map[string]bool{}
		for _, p := range names {
			if !platforms[p] {
				errs = append(errs, fmt.Errorf("%s: unsupported platform %q", field, p))
			}
			if seen[p] {
				errs = append(errs, fmt.Errorf("%s: platform %q is listed twice", field, p))
			}
			seen[p] = true
		}
	}
	check("platforms", s.Platforms)
	keys := make([]string, 0, len(s.Target))
	for k := range s.Target {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	check("target", keys)
	return errs
}

// placeholders returns the {name} placeholders a template uses.
func placeholders(tmpl string) []string {
	var out []string
	for rest := tmpl; ; {
		open := strings.IndexByte(rest, '{')
		if open < 0 {
			return out
		}
		rest = rest[open+1:]
		closed := strings.IndexByte(rest, '}')
		if closed < 0 {
			return out
		}
		out = append(out, rest[:closed])
		rest = rest[closed+1:]
	}
}

func isArchive(name string) bool {
	for _, ext := range archiveExtensions {
		if strings.HasSuffix(name, ext) {
			return true
		}
	}
	return false
}

func validName(name string) error {
	if name == "" {
		return errors.New("name is empty")
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return fmt.Errorf("invalid name %q: use lower-case letters, digits, '-' and '_'", name)
		}
	}
	return nil
}

// validBin mirrors the check block applies to a recipe and to a lockfile: an
// executable path may never leave the directory the artifact unpacks into.
func validBin(b string) error {
	if b == "" {
		return errors.New("bin entry is empty")
	}
	if strings.ContainsAny(b, "\\:") || strings.ContainsRune(b, 0) {
		return fmt.Errorf("invalid bin entry %q: want a relative slash-separated path inside the archive", b)
	}
	clean := path.Clean(b)
	if clean != b || path.IsAbs(b) || clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return fmt.Errorf("invalid bin entry %q: want a relative path inside the archive", b)
	}
	return nil
}

func firstOr(list []string, fallback string) string {
	if len(list) == 0 {
		return fallback
	}
	return list[0]
}
