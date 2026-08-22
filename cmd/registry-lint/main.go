// Command registry-lint checks every recipe in this repository before it can
// reach a block release.
//
// It validates what block itself cannot see — the file name, the ecosystems,
// the description and, above all, where the recipe downloads from — plus the
// shape of the source table, offline and in under a second. What a recipe
// *resolves to* is not checked here: that needs block's resolver, and
// duplicating it would create a second definition of the format. block runs
// those live checks against the snapshot it embeds (see the README).
//
// The download-source rule is the one to look for first. A recipe may take
// its artifact from a GitHub Release of the repository it already names
// (tier 1), or from a host that policy/hosts.toml lists for that repository
// (tier 2). Nothing else passes, so widening what block installs can never
// quietly widen where block downloads from.
//
//	registry-lint                    # lint ./registry against ./policy/hosts.toml
//	registry-lint ../other           # lint another recipe directory
//	registry-lint -hosts h.toml dir  # lint against another policy
package main

import (
	"errors"
	"flag"
	"fmt"
	"maps"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// The source types a recipe may declare, in the order the download-source
// policy prefers them.
const (
	// typeGitHubRelease takes an asset of a release of the same repository the
	// version tags come from, with GitHub's own SHA-256 beside it.
	typeGitHubRelease = "github_release"
	// typeHTTP takes a prebuilt artifact from a host policy/hosts.toml lists.
	typeHTTP = "http"
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
	"windows/amd64": true, "windows/arm64": true,
}

// gitHubHosts serve GitHub's own release assets. Reaching them through type
// "http" would spell out by hand what type "github_release" does properly,
// and would throw away the SHA-256 GitHub publishes beside the asset.
var gitHubHosts = map[string]bool{ //nolint:gochecknoglobals // immutable table
	"github.com": true, "objects.githubusercontent.com": true,
	"release-assets.githubusercontent.com": true, "codeload.github.com": true,
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
	Channels        map[string]channel
}

// channel is one release line an upstream publishes under a tag that moves,
// such as Foundry's nightly. block pins one by dereferencing that tag and
// taking the release published for the commit under it; what a recipe has to
// say is how that release names its assets, because it names them after the
// channel rather than after a version.
type channel struct {
	Asset string
}

func main() {
	hostsPath := flag.String("hosts", filepath.Join("policy", "hosts.toml"), "download-source policy to lint against")
	flag.Parse()
	dir := "registry"
	if flag.NArg() > 0 {
		dir = flag.Arg(0)
	}
	pol, err := loadPolicy(*hostsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "registry-lint: %v\n", err)
		os.Exit(2) //nolint:mnd // 2: could not run, as distinct from "found problems"
	}
	problems, err := lintDir(dir, pol)
	if err != nil {
		fmt.Fprintf(os.Stderr, "registry-lint: %v\n", err)
		os.Exit(2) //nolint:mnd // 2: could not run, as distinct from "found problems"
	}
	problems = append(problems, pol.unused()...)
	for _, p := range problems {
		fmt.Fprintln(os.Stderr, p)
	}
	if len(problems) > 0 {
		fmt.Fprintf(os.Stderr, "\n%d problem(s) in %s\n", len(problems), dir)
		os.Exit(1)
	}
	fmt.Printf("registry-lint: %s\n", pol.summary)
}

// policy is the download-source rule the recipes are held to: the hosts a
// type "http" recipe may name, and for which repository each one is allowed.
type policy struct {
	hosts   []hostRule
	used    map[string]bool
	summary string
}

// hostRule allows exactly one host for exactly one upstream repository. The
// pairing matters: a host that is right for Bitcoin Core is not thereby a
// host any other recipe may reach.
type hostRule struct {
	Host string `toml:"host"`
	Repo string `toml:"repo"`
	Why  string `toml:"why"`
}

// loadPolicy reads the host allowlist. A missing or malformed file stops the
// run rather than silently letting every host through.
func loadPolicy(path string) (*policy, error) {
	data, err := os.ReadFile(path) //nolint:gosec // linting the repository's own files
	if err != nil {
		return nil, fmt.Errorf("download-source policy: %w", err)
	}
	var doc struct {
		Host []hostRule `toml:"host"`
	}
	md, err := toml.Decode(string(data), &doc)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if u := md.Undecoded(); len(u) > 0 {
		return nil, fmt.Errorf("%s: unknown key %q", path, u[0].String())
	}
	seen := map[string]bool{}
	for _, h := range doc.Host {
		switch {
		case h.Host == "":
			return nil, fmt.Errorf("%s: a host entry has no host", path)
		case h.Repo == "":
			return nil, fmt.Errorf("%s: host %q does not say which repository it serves", h.Host, path)
		case h.Why == "":
			return nil, fmt.Errorf("%s: host %q does not say why a GitHub release asset will not do", path, h.Host)
		case gitHubHosts[h.Host]:
			return nil, fmt.Errorf("%s: host %q serves GitHub releases: such a tool must use type \"github_release\"", path, h.Host)
		case seen[h.Host+" "+h.Repo]:
			return nil, fmt.Errorf("%s: host %q is listed twice for %s", path, h.Host, h.Repo)
		}
		seen[h.Host+" "+h.Repo] = true
	}
	return &policy{hosts: doc.Host, used: map[string]bool{}}, nil
}

// allows reports whether repo may download from host, and records the match
// so that an entry no recipe needs can be reported as stale.
func (p *policy) allows(host, repo string) bool {
	for _, h := range p.hosts {
		if h.Host == host && h.Repo == repo {
			p.used[h.Host+" "+h.Repo] = true
			return true
		}
	}
	return false
}

// unused names the policy entries no recipe relies on. They are reported so
// that the allowlist shrinks when a tool moves back to GitHub releases,
// rather than accumulating hosts nothing downloads from any more.
func (p *policy) unused() []string {
	var out []string
	for _, h := range p.hosts {
		if !p.used[h.Host+" "+h.Repo] {
			out = append(out, fmt.Sprintf("policy/hosts.toml: %s is allowed for %s but no recipe downloads from it", h.Host, h.Repo))
		}
	}
	return out
}

// lintDir returns one message per problem, in file order.
func lintDir(dir string, pol *policy) ([]string, error) {
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
	tiers := map[int]int{}
	for _, name := range files {
		errs, tier := lintFile(filepath.Join(dir, name), pol)
		for _, err := range errs {
			problems = append(problems, fmt.Sprintf("%s: %v", name, err))
		}
		tiers[tier]++
	}
	pol.summary = fmt.Sprintf("%d recipe(s) in %s: %d from a GitHub release, %d from an allowed vendor host",
		len(files), dir, tiers[tierGitHubRelease], tiers[tierVendorHost])
	return problems, nil
}

// The tiers of the download-source policy, in the order a recipe should
// prefer them. tierUnknown is what a recipe with a broken source counts as,
// so a summary never claims a source it could not classify.
const (
	tierUnknown = iota
	tierGitHubRelease
	tierVendorHost
)

// lintFile returns the problems in one recipe and which download-source tier
// it belongs to.
func lintFile(path string, pol *policy) ([]error, int) {
	data, err := os.ReadFile(path) //nolint:gosec // linting the repository's own files
	if err != nil {
		return []error{err}, tierUnknown
	}
	var r recipe
	md, err := toml.Decode(string(data), &r)
	if err != nil {
		return []error{err}, tierUnknown
	}
	var errs []error
	for _, key := range md.Undecoded() {
		errs = append(errs, fmt.Errorf("unknown key %q", key.String()))
	}
	errs = append(errs, r.lint(filepath.Base(path))...)
	errs = append(errs, r.Source.lintDownloadSource(pol)...)
	return errs, r.Source.tier()
}

// tier classifies where the recipe downloads from.
func (s source) tier() int {
	switch s.Type {
	case "github_release":
		return tierGitHubRelease
	case "http":
		return tierVendorHost
	default:
		return tierUnknown
	}
}

// lintDownloadSource applies the rule this repository exists to keep: a tool
// is fetched either from a GitHub release of the repository the recipe
// already names, or from a host the policy allows for that repository.
func (s source) lintDownloadSource(pol *policy) []error {
	if s.Type != typeHTTP || s.URL == "" {
		return nil
	}
	// The host is read off the template before any placeholder is expanded,
	// so a templated host is caught here rather than becoming a url only the
	// resolver could tell you about.
	if rest, ok := strings.CutPrefix(s.URL, "https://"); ok {
		authority, _, _ := strings.Cut(rest, "/")
		if strings.ContainsAny(authority, "{}") {
			return []error{fmt.Errorf("url host %q is templated: the host a recipe downloads from must be a constant", authority)}
		}
	}
	u, err := url.Parse(s.URL)
	if err != nil {
		return []error{fmt.Errorf("url template %q is not a url: %w", s.URL, err)}
	}
	host := u.Hostname()
	switch {
	case host == "":
		return []error{fmt.Errorf("url template %q names no host", s.URL)}
	case gitHubHosts[host]:
		return []error{fmt.Errorf("url host %q serves GitHub releases: use type \"github_release\" so the asset's published digest is used", host)}
	case !pol.allows(host, s.Repo):
		return []error{fmt.Errorf("url host %q is not allowed for %s: add it to policy/hosts.toml with the reason a GitHub release asset will not do", host, s.Repo)}
	}
	return nil
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
	case typeGitHubRelease:
		if s.URL != "" {
			errs = append(errs, errors.New(`url is only valid for type "http"`))
		}
		errs = append(errs, s.lintAsset()...)
	case typeHTTP:
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
	errs = append(errs, s.lintChannels()...)
	return errs
}

// lintChannels checks the moving release lines a source declares.
func (s source) lintChannels() []error {
	var errs []error
	for _, name := range slices.Sorted(maps.Keys(s.Channels)) {
		ch := s.Channels[name]
		if !validChannelName(name) {
			errs = append(errs, fmt.Errorf("channel %q: use lower-case letters, digits and '-', starting with a letter", name))
		}
		if s.Type != typeGitHubRelease {
			errs = append(errs, fmt.Errorf("channel %q: channels need type %q", name, typeGitHubRelease))
			continue
		}
		switch {
		case ch.Asset == "":
			errs = append(errs, fmt.Errorf("channel %q: asset template is required", name))
			continue
		case strings.ContainsAny(ch.Asset, "/\\"):
			errs = append(errs, fmt.Errorf("channel %q: asset template %q must be a bare file name", name, ch.Asset))
		case strings.Contains(ch.Asset, "{version}"):
			errs = append(errs, fmt.Errorf("channel %q: asset template %q uses {version}, and a channel release has no version", name, ch.Asset))
		case isArchive(ch.Asset) != isArchive(s.Asset):
			errs = append(errs, fmt.Errorf("channel %q: asset %q is not the same kind of artifact as %q", name, ch.Asset, s.Asset))
		case strings.Contains(ch.Asset, "{target}") && len(s.Target) == 0:
			errs = append(errs, fmt.Errorf("channel %q: {target} needs a [source.target] table", name))
		}
		for _, ph := range placeholders(ch.Asset) {
			switch ph {
			case "os", "arch", "commit", "target":
			default:
				errs = append(errs, fmt.Errorf("channel %q: unknown placeholder {%s}", name, ph))
			}
		}
	}
	return errs
}

// validChannelName accepts the names an upstream gives a release line.
func validChannelName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
		case i > 0 && (r >= '0' && r <= '9' || r == '-'):
		default:
			return false
		}
	}
	return true
}

func (s source) lintAsset() []error {
	var errs []error
	if s.Asset == "" {
		return []error{errors.New("asset template is required")}
	}
	if strings.ContainsAny(s.Asset, "/\\") {
		errs = append(errs, fmt.Errorf("asset template %q must be a bare file name", s.Asset))
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
