package vcs

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kyleking/aragonite/cache"
)

// Stamp fingerprints what the checkout at repoPath looks like right now,
// without spawning a subprocess: the OID HEAD resolves to, the current branch
// and the upstream it tracks, that upstream's remote-tracking OID, and the
// mtimes of the ref files a commit, switch, push, or fetch touches. Cheap
// enough to take on every render. It stats refs/heads and refs/remotes/<remote>
// directly, since a directory's mtime tracks only its own entries.
//
// A jj repo is stamped on its git refs alone, which a colocated repo keeps in
// step with every operation. A workspace with no .git yields cache.NoStamp,
// which proves nothing and so leaves values derived from it uncached rather
// than served stale.
func Stamp(repoPath string) cache.Stamp {
	gitDir, ok := resolveGitDir(repoPath)
	if !ok {
		return cache.NoStamp
	}

	refs := &refResolver{commonDir: commonGitDir(gitDir)}

	head := readHead(gitDir, refs)
	if head.oid == "" && head.branch == "" {
		return cache.NoStamp
	}

	upstream := upstreamRef(refs.commonDir, head.branch)

	remotesDir := filepath.Join(refs.commonDir, "refs", "remotes")
	if upstream.remote != "" {
		remotesDir = filepath.Join(remotesDir, upstream.remote)
	}

	fields := []string{
		head.oid,
		head.branch,
		upstream.ref,
		refs.resolve(upstream.ref),
		strconv.FormatInt(modNanos(filepath.Join(refs.commonDir, "packed-refs")), 10),
		strconv.FormatInt(modNanos(filepath.Join(refs.commonDir, "FETCH_HEAD")), 10),
		strconv.FormatInt(modNanos(filepath.Join(refs.commonDir, "refs", "heads")), 10),
		strconv.FormatInt(modNanos(remotesDir), 10),
	}

	return cache.Stamp{Scope: CheckoutIdentity(repoPath), Fingerprint: strings.Join(fields, "\x00")}
}

// resolveGitDir returns the directory holding repoPath's own HEAD, following
// the "gitdir:" pointer a linked worktree keeps in place of a .git directory.
func resolveGitDir(repoPath string) (string, bool) {
	path := filepath.Join(repoPath, ".git")

	info, err := os.Stat(path)
	if err != nil {
		return "", false
	}

	if info.IsDir() {
		return path, true
	}

	contents, err := os.ReadFile(path) // #nosec G304 -- path is the repo's own .git pointer
	if err != nil {
		return "", false
	}

	gitDir, ok := strings.CutPrefix(strings.TrimSpace(string(contents)), "gitdir:")
	if !ok {
		return "", false
	}

	return filepath.Clean(strings.TrimSpace(gitDir)), true
}

// commonGitDir returns the directory holding the refs gitDir reads, which for
// a linked worktree at <common>/worktrees/<name> is the common directory two
// levels up.
func commonGitDir(gitDir string) string {
	if filepath.Base(filepath.Dir(gitDir)) == "worktrees" {
		return filepath.Dir(filepath.Dir(gitDir))
	}

	return gitDir
}

// headState is what HEAD names: the OID it resolves to, and the branch it
// points at, empty when HEAD is detached.
type headState struct{ oid, branch string }

func readHead(gitDir string, refs *refResolver) headState {
	contents, err := os.ReadFile(filepath.Join(gitDir, "HEAD")) // #nosec G304 -- path is the repo's own HEAD
	if err != nil {
		return headState{}
	}

	line := strings.TrimSpace(string(contents))

	ref, ok := strings.CutPrefix(line, "ref:")
	if !ok {
		return headState{oid: line}
	}

	name := strings.TrimSpace(ref)

	return headState{oid: refs.resolve(name), branch: strings.TrimPrefix(name, "refs/heads/")}
}

// tracking names the remote a branch tracks and the ref that follows it: the
// remote-tracking ref for a real remote, the local branch for a "." remote.
type tracking struct{ remote, ref string }

// upstreamRef returns what branch tracks, zero when it tracks nothing.
func upstreamRef(commonDir, branch string) tracking {
	if branch == "" {
		return tracking{}
	}

	configured := branchConfig(commonDir, branch)
	if configured.remote == "" || configured.ref == "" {
		return tracking{}
	}

	if configured.remote == "." {
		return tracking{ref: configured.ref}
	}

	return tracking{
		remote: configured.remote,
		ref:    "refs/remotes/" + configured.remote + "/" + strings.TrimPrefix(configured.ref, "refs/heads/"),
	}
}

// branchConfig reads the remote and merge ref configured for branch out of the
// repository's own config, ignoring the includes and system-wide files git
// would also consult: an upstream set anywhere but here is vanishingly rare,
// and a wrong answer costs a cache miss rather than a wrong value.
func branchConfig(commonDir, branch string) tracking {
	var configured tracking

	configSection(filepath.Join(commonDir, "config"), `branch "`+branch+`"`,
		func(key, value string) {
			switch key {
			case "remote":
				configured.remote = value
			case "merge":
				configured.ref = value
			}
		})

	return configured
}

// remoteURLOrigin is the url `git remote get-url origin` would answer, read
// straight from the config the subprocess would parse, under the same "the
// repo's own config only" rule as branchConfig.
func remoteURLOrigin(configDir string) string {
	var url string

	configSection(filepath.Join(configDir, "config"), `remote "origin"`,
		func(key, value string) {
			if key == "url" && url == "" {
				url = value
			}
		})

	return url
}

// configSection calls keep for each key=value line inside the named section
// of the config at path. Sections match the "[name "sub"]" form git writes.
func configSection(path, section string, keep func(key, value string)) {
	file, err := os.Open(path) // #nosec G304 -- path is the repo's own config
	if err != nil {
		return
	}
	defer file.Close() //nolint:errcheck // read-only handle

	want := "[" + section + "]"
	inSection := false
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") {
			inSection = line == want

			continue
		}

		if !inSection {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		keep(strings.TrimSpace(key), strings.TrimSpace(value))
	}
}

// gitConfigDir returns the directory holding the config a checkout's refs
// follow: its own .git, or the common directory a linked worktree points
// back to.
func gitConfigDir(repoPath string) (string, bool) {
	gitDir, ok := resolveGitDir(repoPath)
	if !ok {
		return "", false
	}

	return commonGitDir(gitDir), true
}

// jjGitDir returns the git store a jj repo keeps its refs and remotes in:
// <root>/.jj/repo/store/git for the primary checkout, the same store behind
// the repo pointer file a secondary workspace keeps instead.
func jjGitDir(repoPath string) (string, bool) {
	repo := filepath.Join(repoPath, ".jj", "repo")

	info, err := os.Stat(repo)
	if err != nil {
		return "", false
	}

	if info.IsDir() {
		return filepath.Join(repo, "store", "git"), true
	}

	contents, err := os.ReadFile(repo) // #nosec G304 -- a repo's own .jj pointer
	if err != nil {
		return "", false
	}

	return filepath.Join(strings.TrimSpace(string(contents)), "store", "git"), true
}

// refResolver reads ref OIDs out of a repository's common directory, loading
// packed-refs at most once.
type refResolver struct {
	packed    map[string]string
	commonDir string
}

func (r *refResolver) resolve(ref string) string {
	if ref == "" {
		return ""
	}

	refPath := filepath.Join(r.commonDir, filepath.FromSlash(ref))

	loose, err := os.ReadFile(refPath) // #nosec G304,G703 -- a ref path under the repo's own git dir
	if err == nil {
		return strings.TrimSpace(string(loose))
	}

	if r.packed == nil {
		r.packed = readPackedRefs(r.commonDir)
	}

	return r.packed[ref]
}

func readPackedRefs(commonDir string) map[string]string {
	packed := make(map[string]string)

	file, err := os.Open(filepath.Join(commonDir, "packed-refs")) // #nosec G304 -- path is the repo's own packed-refs
	if err != nil {
		return packed
	}
	defer file.Close() //nolint:errcheck // read-only handle

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || line[0] == '#' || line[0] == '^' {
			continue
		}

		oid, ref, ok := strings.Cut(line, " ")
		if ok {
			packed[strings.TrimSpace(ref)] = oid
		}
	}

	return packed
}

func modNanos(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}

	return info.ModTime().UnixNano()
}
