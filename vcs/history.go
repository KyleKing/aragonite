package vcs

import (
	"bufio"
	"context"
	"crypto/sha1" //nolint:gosec // git object ids are sha1 by definition, not a security choice
	"encoding/hex"
	"fmt"
	"io"
	"iter"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ChangeStatus names what a commit did to one file.
type ChangeStatus int

// ChangeStatus values.
const (
	StatusModified ChangeStatus = iota
	StatusAdded
	StatusDeleted
	StatusRenamed
)

func (c ChangeStatus) String() string {
	switch c {
	case StatusAdded:
		return "added"
	case StatusDeleted:
		return "deleted"
	case StatusRenamed:
		return "renamed"
	case StatusModified:
		return "modified"
	default:
		return unknownLabel
	}
}

// DiffHunk is one changed span, expressed against both sides of a diff. A side
// with zero lines is purely added or purely removed there.
type DiffHunk struct {
	// OldStart with OldLines 0 is the line after the anchor, so OldStart-1 is the insert index.
	OldStart int
	OldLines int
	// NewStart with NewLines 0 is the line after the anchor, so NewStart-1 is the insert index.
	NewStart int
	NewLines int
}

// CommitFile is one file a commit or a worktree diff touched.
type CommitFile struct {
	Path    string
	OldPath string // set on renames only
	OldSHA  string // zero-padded when the side does not exist
	NewSHA  string
	// Added and Removed hold the + and - line texts of the patch, per file
	// rather than per hunk so a caller matching moved code across hunks has
	// one list. Context lines are not kept; -U0 callers emit none.
	Added   []string
	Removed []string
	Hunks   []DiffHunk
	Status  ChangeStatus
	Binary  bool
}

// LinesAdded is the total of + lines across hunks.
func (f CommitFile) LinesAdded() int {
	n := 0
	for _, h := range f.Hunks {
		n += h.NewLines
	}

	return n
}

// LinesDeleted is the total of - lines across hunks.
func (f CommitFile) LinesDeleted() int {
	n := 0
	for _, h := range f.Hunks {
		n += h.OldLines
	}

	return n
}

// CommitRecord is one commit on a history walk, with the diff it made against
// its first parent.
type CommitRecord struct {
	AuthorTime     time.Time
	CommitTime     time.Time
	Hash           string
	AuthorName     string
	AuthorEmail    string
	CommitterName  string
	CommitterEmail string
	Subject        string
	Body           string
	Parents        []string
	Files          []CommitFile
}

// TreeEntry is one blob in a revision's tree.
type TreeEntry struct {
	Path string
	SHA  string
	Mode string
	Size int64
}

// LogOptions shapes a LogHistory walk. The zero value walks every commit
// reachable from HEAD, newest first.
type LogOptions struct {
	Range       string   // a revision range like "main..HEAD", or a single rev
	Since       string   // passed to --since verbatim, e.g. "90 days ago"
	Paths       []string // restrict the walk and its diffs to these paths
	FirstParent bool     // follow only the first-parent chain
	Reverse     bool     // oldest first
}

// readBufSize is the stdin/stdout pipe buffer for streaming git output.
const readBufSize = 1 << 20

// recordSep opens each commit's header line. Patch lines start with a diff
// prefix, so only a header line can begin a record.
const recordSep = "\x1e"

// logFormat carries every field LogHistory needs, NUL-separated, with the
// patch following the last field.
const logFormat = recordSep + "%H%x00%P%x00%an%x00%ae%x00%cn%x00%ce%x00%aI%x00%cI%x00%s%x00%b%x00"

// hunkHeader matches @@ -old[,count] +new[,count] @@.
//
//nolint:gochecknoglobals // a compiled pattern is a constant
var hunkHeader = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

// MergeBase returns the best common ancestor commit of a and b.
func (g *GitOperations) MergeBase(ctx context.Context, repoPath, a, b string) (string, error) {
	return g.runGit(ctx, repoPath, "merge-base", a, b)
}

// ResolveRef resolves a revision expression to its full commit SHA.
func (g *GitOperations) ResolveRef(ctx context.Context, repoPath, rev string) (string, error) {
	return g.runGit(ctx, repoPath, "rev-parse", "--verify", rev+"^{commit}")
}

// DefaultBranchName answers the repository's primary branch: origin/HEAD's
// target when set, else the first conventional name that resolves.
func (g *GitOperations) DefaultBranchName(ctx context.Context, repoPath string) (string, error) {
	ref, err := g.runGit(ctx, repoPath, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
	if err == nil {
		if branch, ok := strings.CutPrefix(ref, "origin/"); ok {
			return branch, nil
		}
	}

	for _, name := range DefaultBranchNames {
		if _, err := g.runGit(ctx, repoPath, "rev-parse", "--verify", "refs/heads/"+name); err == nil {
			return name, nil
		}
	}

	return "", fmt.Errorf("no default branch found: %w", ErrCommandFailed)
}

// BlobContent returns one blob's bytes.
func (g *GitOperations) BlobContent(ctx context.Context, repoPath, sha string) ([]byte, error) {
	out, err := g.runGitRaw(ctx, repoPath, "cat-file", "blob", sha)
	if err != nil {
		return nil, err
	}

	return []byte(out), nil
}

// BlobContents reads many blobs through one cat-file --batch process. A sha
// git does not know, including an all-zero sha, is absent from the result.
func (*GitOperations) BlobContents(ctx context.Context, repoPath string, shas []string) (map[string][]byte, error) {
	want := make([]string, 0, len(shas))
	for _, sha := range shas {
		if sha != "" && !isZeroSHA(sha) {
			want = append(want, sha)
		}
	}
	out := make(map[string][]byte, len(want))
	if len(want) == 0 {
		return out, nil
	}

	cmd := exec.CommandContext(ctx, "git", "cat-file", "--batch") // #nosec G204 -- fixed argv
	cmd.Dir = repoPath
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("git cat-file --batch: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("git cat-file --batch: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, gitError([]string{"cat-file", "--batch"}, err)
	}

	go func() {
		for _, sha := range want {
			// A write error means the reader already saw the process die; the
			// error surfaces through cmd.Wait below.
			_, _ = fmt.Fprintln(stdin, sha) //nolint:errcheck // see comment
		}
		_ = stdin.Close() //nolint:errcheck // closing signals EOF; git then exits
	}()

	r := bufio.NewReaderSize(stdout, readBufSize)
	readErr := readBatch(r, want, out)
	waitErr := cmd.Wait()
	if readErr != nil {
		return nil, readErr
	}
	if waitErr != nil {
		return nil, gitError([]string{"cat-file", "--batch"}, waitErr)
	}

	return out, nil
}

// readBatch parses cat-file --batch output: a "<sha> <type> <size>" header,
// the payload, and one trailing newline per object. Answers arrive in request
// order, so each response zips with want[i]; the header sha is the resolved
// full sha, which is not what the caller asked for when it passed an
// abbreviated one, so the map is keyed by the request.
func readBatch(r *bufio.Reader, want []string, out map[string][]byte) error {
	for _, sha := range want {
		header, err := r.ReadString('\n')
		if err != nil {
			return fmt.Errorf("git cat-file --batch: short response: %w", err)
		}
		fields := strings.Fields(strings.TrimRight(header, "\n"))
		if len(fields) < 2 { //nolint:mnd // header is at least "<sha> <type>"
			return fmt.Errorf("git cat-file --batch: %q: %w", header, ErrUnexpectedOutput)
		}
		if len(fields) == 2 && fields[1] == "missing" {
			continue
		}
		if len(fields) < 3 { //nolint:mnd // a present object needs a size
			return fmt.Errorf("git cat-file --batch: %q: %w", header, ErrUnexpectedOutput)
		}
		size, err := strconv.Atoi(fields[2])
		if err != nil {
			return fmt.Errorf("git cat-file --batch: %q: %w", header, ErrUnexpectedOutput)
		}
		payload := make([]byte, size+1)
		if _, err := io.ReadFull(r, payload); err != nil {
			return fmt.Errorf("git cat-file --batch: %w", err)
		}
		out[sha] = payload[:size]
	}

	return nil
}

// TreeEntries lists every blob in rev's tree, recursively, with sizes. Submodule entries are skipped.
func (g *GitOperations) TreeEntries(ctx context.Context, repoPath, rev string) ([]TreeEntry, error) {
	out, err := g.runGitRaw(ctx, repoPath, "ls-tree", "-r", "-l", "-z", rev)
	if err != nil {
		return nil, err
	}

	var entries []TreeEntry
	for record := range strings.SplitSeq(out, "\x00") {
		if record == "" {
			continue
		}
		meta, path, ok := strings.Cut(record, "\t")
		if !ok {
			return nil, fmt.Errorf("git ls-tree: %q: %w", record, ErrUnexpectedOutput)
		}
		fields := strings.Fields(meta)
		if len(fields) != 4 { //nolint:mnd // mode, type, sha, size
			return nil, fmt.Errorf("git ls-tree: %q: %w", record, ErrUnexpectedOutput)
		}
		if fields[1] == "commit" {
			continue
		}
		size, err := strconv.ParseInt(fields[3], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("git ls-tree: %q: %w", record, ErrUnexpectedOutput)
		}
		entries = append(entries, TreeEntry{Path: path, SHA: fields[2], Mode: fields[0], Size: size})
	}

	return entries, nil
}

// DiffRefs diffs base against head with renames detected and zero context, so
// each hunk names exactly the lines that moved.
func (g *GitOperations) DiffRefs(ctx context.Context, repoPath, base, head string) ([]CommitFile, error) {
	out, err := g.runGitRaw(ctx, repoPath, "-c", "core.quotePath=false",
		"diff", "--no-ext-diff", "--no-textconv", "-U0", "-M", "--no-color", base, head)
	if err != nil {
		return nil, err
	}

	return ParsePatch(out), nil
}

// DiffWorktree diffs base against the working tree, covering staged and
// unstaged edits to tracked files. Untracked files are not in the diff;
// UntrackedFiles names them for the caller to score as additions.
func (g *GitOperations) DiffWorktree(ctx context.Context, repoPath, base string) ([]CommitFile, error) {
	out, err := g.runGitRaw(ctx, repoPath, "-c", "core.quotePath=false",
		"diff", "--no-ext-diff", "--no-textconv", "-U0", "-M", "--no-color", base)
	if err != nil {
		return nil, err
	}

	return ParsePatch(out), nil
}

// UntrackedFiles lists worktree files git does not track, honoring .gitignore.
func (g *GitOperations) UntrackedFiles(ctx context.Context, repoPath string) ([]string, error) {
	out, err := g.runGitRaw(ctx, repoPath, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}

	var paths []string
	for p := range strings.SplitSeq(out, "\x00") {
		if p != "" {
			paths = append(paths, p)
		}
	}

	return paths, nil
}

// LogHistory streams commits matching opts along with the patch each made.
// The sequence stops early without leaking the git process when the consumer
// breaks out of range.
func (*GitOperations) LogHistory(ctx context.Context, repoPath string, opts LogOptions) iter.Seq2[CommitRecord, error] {
	return func(yield func(CommitRecord, error) bool) {
		args := logArgs(opts)

		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		cmd := exec.CommandContext(ctx, "git", args...) // #nosec G204 -- argv from callers, not users
		cmd.Dir = repoPath
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			yield(CommitRecord{}, fmt.Errorf("git log: %w", err))

			return
		}
		if err := cmd.Start(); err != nil {
			yield(CommitRecord{}, gitError(args, err))

			return
		}

		r := bufio.NewReaderSize(stdout, readBufSize)
		for raw := range splitRecords(r) {
			if !yield(parseLogRecord(raw), nil) {
				cancel()
				_ = cmd.Wait() //nolint:errcheck // the consumer stopped, so no one reads the exit status

				return
			}
		}

		if err := cmd.Wait(); err != nil {
			yield(CommitRecord{}, gitError(args, err))
		}
	}
}

// logArgs builds the git log invocation for a walk. -m keeps a merge's
// first-parent diff, -U0 makes hunks name only changed lines, and -M keeps
// renames from reading as a delete plus an add.
func logArgs(opts LogOptions) []string {
	args := []string{
		"-c", "core.quotePath=false", "log", "-m", "--no-ext-diff", "--no-textconv",
		"-U0", "-M", "--no-color", "--format=" + logFormat,
	}
	if opts.FirstParent {
		args = append(args, "--first-parent")
	}
	if opts.Reverse {
		args = append(args, "--reverse")
	}
	if opts.Since != "" {
		args = append(args, "--since="+opts.Since)
	}
	if opts.Range != "" {
		args = append(args, opts.Range)
	}
	if len(opts.Paths) > 0 {
		args = append(args, "--")
		args = append(args, opts.Paths...)
	}

	return args
}

// splitRecords yields each commit's raw text, header through patch, from a git log stream.
func splitRecords(r *bufio.Reader) iter.Seq[string] {
	return func(yield func(string) bool) {
		var record strings.Builder
		for {
			line, err := r.ReadString('\n')
			if isRecordStart(line) {
				if record.Len() > 0 && !yield(record.String()) {
					return
				}
				record.Reset()
				line = strings.TrimPrefix(line, recordSep)
			}
			record.WriteString(line)
			if err != nil {
				break
			}
		}
		if record.Len() > 0 {
			yield(record.String())
		}
	}
}

// isRecordStart reports whether line opens a commit header: recordSep, then a
// hash ending in NUL.
func isRecordStart(line string) bool {
	rest, ok := strings.CutPrefix(line, recordSep)
	if !ok {
		return false
	}
	hash, _, found := strings.Cut(rest, "\x00")

	return found && hash != "" && strings.Trim(hash, "0123456789abcdef") == ""
}

// parseLogRecord splits one record into its metadata fields and its patch.
func parseLogRecord(record string) CommitRecord {
	fields := strings.SplitN(record, "\x00", 11) //nolint:mnd // 10 fields plus the patch
	rec := CommitRecord{}
	if len(fields) < 11 { //nolint:mnd // see above
		return rec
	}

	rec.Hash = fields[0]
	if fields[1] != "" {
		rec.Parents = strings.Fields(fields[1])
	}
	rec.AuthorName = fields[2]
	rec.AuthorEmail = fields[3]
	rec.CommitterName = fields[4]
	rec.CommitterEmail = fields[5]
	rec.AuthorTime, _ = time.Parse(time.RFC3339, fields[6]) //nolint:errcheck // git emits RFC3339 here by contract
	rec.CommitTime, _ = time.Parse(time.RFC3339, fields[7]) //nolint:errcheck // same
	rec.Subject = fields[8]
	rec.Body = fields[9]
	rec.Files = ParsePatch(fields[10])

	return rec
}

// patchParser accumulates CommitFiles out of a unified-diff line stream.
type patchParser struct {
	cur          *CommitFile
	aPath, bPath string
	header       string
	files        []CommitFile
	// inHunks is set at the first @@ of a file; after it, ---/+++ lines are removed or added text.
	inHunks bool
}

// patchLines maps each diff-header prefix to its effect on the file in
// flight. No key is a prefix of another, so a map lookup per line is safe.
var patchLines = map[string]func(*patchParser, string){ //nolint:gochecknoglobals // a parse table is a constant
	"new file mode":     func(p *patchParser, _ string) { p.cur.Status = StatusAdded },
	"deleted file mode": func(p *patchParser, _ string) { p.cur.Status = StatusDeleted },
	"rename from ": func(p *patchParser, l string) {
		p.cur.Status = StatusRenamed
		p.cur.OldPath = strings.TrimPrefix(l, "rename from ")
	},
	"rename to ": func(p *patchParser, l string) {
		p.cur.Status = StatusRenamed
		p.cur.Path = strings.TrimPrefix(l, "rename to ")
	},
	"index ":           func(p *patchParser, l string) { p.cur.OldSHA, p.cur.NewSHA = parseIndexLine(l) },
	"Binary files ":    func(p *patchParser, _ string) { p.cur.Binary = true },
	"GIT binary patch": func(p *patchParser, _ string) { p.cur.Binary = true },
	"--- ":             func(p *patchParser, l string) { p.aPath = patchPath(l[4:]) },
	"+++ ":             func(p *patchParser, l string) { p.bPath = patchPath(l[4:]) },
}

// ParsePatch parses unified-diff text of the kind `git diff` and `git log -p`
// emit, expecting zero context (-U0) so hunks list only changed lines.
func ParsePatch(text string) []CommitFile {
	p := &patchParser{}
	for line := range strings.Lines(text) {
		p.line(strings.TrimSuffix(line, "\n"))
	}
	p.finish()

	return p.files
}

func (p *patchParser) line(line string) {
	if strings.HasPrefix(line, "diff --git ") {
		p.finish()
		p.cur = &CommitFile{}
		p.header = line

		return
	}
	if p.cur == nil {
		return // text before the first diff --git, such as a commit header
	}
	if strings.HasPrefix(line, "@@ ") {
		p.inHunks = true
		if h, ok := parseHunk(line); ok {
			p.cur.Hunks = append(p.cur.Hunks, h)
		}

		return
	}
	if !p.inHunks {
		for prefix, apply := range patchLines {
			if strings.HasPrefix(line, prefix) {
				apply(p, line)

				return
			}
		}
	}
	if strings.HasPrefix(line, "+") {
		p.cur.Added = append(p.cur.Added, line[1:])
	}
	if strings.HasPrefix(line, "-") {
		p.cur.Removed = append(p.cur.Removed, line[1:])
	}
}

func (p *patchParser) finish() {
	if p.cur == nil {
		return
	}
	if p.cur.Path == "" {
		p.cur.Path = p.bPath
	}
	if p.cur.Path == "" {
		p.cur.Path = p.aPath
	}
	if p.cur.Path == "" {
		p.cur.Path = diffHeaderPath(p.header)
	}
	if p.cur.OldPath == "" && p.cur.Status == StatusRenamed {
		p.cur.OldPath = p.aPath
	}
	p.files = append(p.files, *p.cur)
	p.cur = nil
	p.aPath, p.bPath, p.header = "", "", ""
	p.inHunks = false
}

// diffHeaderPath reads the path from a "diff --git a/<p> b/<p>" line. Binary
// files have no ---/+++ lines, so this line is the only place their name appears.
func diffHeaderPath(line string) string {
	rest, ok := strings.CutPrefix(line, "diff --git ")
	if !ok {
		return ""
	}
	if strings.HasPrefix(rest, `"`) {
		quoted, err := strconv.QuotedPrefix(rest)
		if err != nil {
			return ""
		}
		name, err := strconv.Unquote(quoted)
		if err != nil {
			return ""
		}

		return strings.TrimPrefix(name, "a/")
	}

	body, ok := strings.CutPrefix(rest, "a/")
	if !ok {
		return ""
	}
	n := len(body) - len(" b/")
	if n < 0 || n%2 != 0 {
		return ""
	}
	name := body[:n/2]
	if body[n/2:] != " b/"+name {
		return ""
	}

	return name
}

// patchPath extracts a filename from a ---/+++ line, honoring git's convention
// that a path containing spaces is followed by a trailing tab. /dev/null and
// rename-only blocks (which carry no ---/+++ lines) answer "".
func patchPath(field string) string {
	name := strings.TrimSuffix(field, "\t")
	if name == "/dev/null" {
		return ""
	}
	if rest, ok := strings.CutPrefix(name, "a/"); ok {
		return rest
	}

	return strings.TrimPrefix(name, "b/")
}

// parseIndexLine reads "index <old>..<new>[,mode]" into the two blob ids.
func parseIndexLine(line string) (string, string) {
	body := strings.TrimPrefix(line, "index ")
	body, _, _ = strings.Cut(body, " ")
	oldSHA, newSHA, _ := strings.Cut(body, "..")

	return oldSHA, newSHA
}

// parseHunk reads "@@ -a[,b] +c[,d] @@" into a DiffHunk.
func parseHunk(line string) (DiffHunk, bool) {
	m := hunkHeader.FindStringSubmatch(line)
	if m == nil {
		return DiffHunk{}, false
	}
	atoi := func(s string, fallback int) int {
		if s == "" {
			return fallback
		}
		n, _ := strconv.Atoi(s) //nolint:errcheck // the regex only admits digits

		return n
	}

	h := DiffHunk{
		OldStart: atoi(m[1], 0),
		OldLines: atoi(m[2], 1),
		NewStart: atoi(m[3], 0),
		NewLines: atoi(m[4], 1),
	}
	if h.OldLines == 0 {
		h.OldStart++
	}
	if h.NewLines == 0 {
		h.NewStart++
	}

	return h, true
}

// isZeroSHA reports whether sha is all zeros, the marker git uses for a side
// that does not exist.
func isZeroSHA(sha string) bool {
	return strings.Trim(sha, "0") == ""
}

// BlobSHA computes the git object id content would have as a blob, without
// writing to any object database.
func BlobSHA(content []byte) string {
	h := sha1.New()                                    //nolint:gosec // git object ids are sha1
	_, _ = fmt.Fprintf(h, "blob %d\x00", len(content)) //nolint:errcheck // a hash never fails Write
	h.Write(content)

	return hex.EncodeToString(h.Sum(nil))
}
