package vcs_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/kyleking/aragonite/vcs"
)

// newGitRepo returns an empty repository, a helper that runs git in it, and a
// helper that writes a file under it.
func newGitRepo(t *testing.T) (string, func(...string), func(string, string)) {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}

	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()

		cmd := exec.CommandContext(t.Context(), "git", args...) // #nosec G204 -- args are literals from this test
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
			"GIT_AUTHOR_DATE=2026-01-01T00:00:00Z",
		)

		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(name, body string) {
		t.Helper()

		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", name, err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}

	return dir, run, write
}

// historyRepo builds a small repo covering every path the walker has: a plain
// modify, a new file, a rename, a spaced filename, a binary file, and a merge
// whose first-parent diff is the one -m reports.
//
//	main:  c1 -- c2 -- c5 -- merge(parents c5+c4)
//	side:        c3 -- c4
func historyRepo(t *testing.T) string {
	t.Helper()

	dir, run, write := newGitRepo(t)

	run("init", "--initial-branch=main")
	write("f.txt", "one\ntwo\nthree\n")
	write("dir with space/s.txt", "space\n")
	write("bin.dat", "\x00\x01\x02")
	run("add", "-A")
	run("commit", "-m", "c1")

	run("checkout", "-b", "side")
	write("side.txt", "side\n")
	run("add", "-A")
	run("commit", "-m", "c3")
	write("side.txt", "side\nmore\n")
	run("commit", "-am", "c4")

	run("checkout", "main")
	write("f.txt", "one\nTWO\nthree\n")
	run("mv", "dir with space/s.txt", "renamed.txt")
	run("add", "-A")
	run("commit", "-m", "c2")
	write("new.txt", "new\n")
	run("add", "-A")
	run("commit", "-m", "c5")

	run("merge", "--no-ff", "-m", "merge side", "side")

	return dir
}

func collect(t *testing.T, seq func(yield func(vcs.CommitRecord, error) bool)) []vcs.CommitRecord {
	t.Helper()

	var out []vcs.CommitRecord
	for rec, err := range seq {
		if err != nil {
			t.Fatalf("LogHistory: %v", err)
		}
		out = append(out, rec)
	}

	return out
}

func TestRefsAgainstRealGit(t *testing.T) {
	t.Parallel()

	dir := historyRepo(t)
	ops := vcs.NewGitOperations()
	ctx := t.Context()

	branch, err := ops.DefaultBranchName(ctx, dir)
	if err != nil {
		t.Fatalf("DefaultBranchName: %v", err)
	}
	if branch != "main" {
		t.Fatalf("DefaultBranchName = %q, want main", branch)
	}

	head, err := ops.ResolveRef(ctx, dir, "HEAD")
	if err != nil {
		t.Fatalf("ResolveRef: %v", err)
	}
	c1, err := ops.ResolveRef(ctx, dir, "HEAD~3")
	if err != nil {
		t.Fatalf("ResolveRef HEAD~3: %v", err)
	}
	preMerge, err := ops.MergeBase(ctx, dir, "HEAD~2", "side")
	if err != nil {
		t.Fatalf("MergeBase HEAD~2..side: %v", err)
	}
	if preMerge != c1 {
		t.Fatalf("merge-base = %q, want c1 %q", preMerge, c1)
	}

	// Once side is merged its tip is itself the common ancestor.
	sideTip, err := ops.ResolveRef(ctx, dir, "side")
	if err != nil {
		t.Fatalf("ResolveRef side: %v", err)
	}
	base, err := ops.MergeBase(ctx, dir, "main", "side")
	if err != nil {
		t.Fatalf("MergeBase: %v", err)
	}
	if base != sideTip || base == head {
		t.Fatalf("merge-base(main, side) = %q, want side tip %q", base, sideTip)
	}
}

func TestLogHistoryFirstParent(t *testing.T) {
	t.Parallel()

	dir := historyRepo(t)
	ops := vcs.NewGitOperations()

	head, err := ops.ResolveRef(t.Context(), dir, "HEAD")
	if err != nil {
		t.Fatalf("ResolveRef: %v", err)
	}

	records := collect(t, ops.LogHistory(t.Context(), dir, vcs.LogOptions{FirstParent: true}))
	if len(records) != 4 {
		t.Fatalf("got %d first-parent records, want 4 (c1, c2, c5, merge)", len(records))
	}

	merge := records[0]
	if merge.Hash != head || len(merge.Parents) != 2 {
		t.Fatalf("merge record = %+v", merge)
	}
	// The merge's first-parent diff brings in exactly the side branch's file.
	if len(merge.Files) != 1 || merge.Files[0].Path != "side.txt" || merge.Files[0].LinesAdded() != 2 {
		t.Fatalf("merge files = %+v, want one side.txt adding 2", merge.Files)
	}
	if merge.AuthorName != "test" || merge.Subject != "merge side" {
		t.Fatalf("merge metadata = %q %q", merge.AuthorName, merge.Subject)
	}
}

func TestLogHistoryFileDetail(t *testing.T) {
	t.Parallel()

	dir := historyRepo(t)
	ops := vcs.NewGitOperations()

	records := collect(t, ops.LogHistory(t.Context(), dir, vcs.LogOptions{FirstParent: true}))
	bySubject := map[string]vcs.CommitRecord{}
	for _, rec := range records {
		bySubject[rec.Subject] = rec
	}

	c2 := bySubject["c2"]
	if len(c2.Files) != 2 { // c2 modifies f.txt and renames s.txt
		t.Fatalf("c2 files = %+v", c2.Files)
	}
	var rename *vcs.CommitFile
	for i := range c2.Files {
		if c2.Files[i].Status == vcs.StatusRenamed {
			rename = &c2.Files[i]
		}
	}
	if rename == nil || rename.OldPath != "dir with space/s.txt" || rename.Path != "renamed.txt" {
		t.Fatalf("rename = %+v", rename)
	}
	for _, f := range c2.Files {
		if f.Status == vcs.StatusModified && (f.LinesAdded() != 1 || f.LinesDeleted() != 1) {
			t.Fatalf("f.txt diff = %+v", f)
		}
	}

	if c5 := bySubject["c5"]; len(c5.Files) != 1 || c5.Files[0].Status != vcs.StatusAdded {
		t.Fatalf("c5 files = %+v", c5.Files)
	}
}

func TestLogHistoryOrderAndRange(t *testing.T) {
	t.Parallel()

	dir := historyRepo(t)
	ops := vcs.NewGitOperations()

	reversed := collect(t, ops.LogHistory(t.Context(), dir,
		vcs.LogOptions{FirstParent: true, Reverse: true}))
	if len(reversed) != 4 || reversed[0].Subject != "c1" {
		t.Fatalf("reverse walk starts with %+v", reversed)
	}

	ranged := collect(t, ops.LogHistory(t.Context(), dir,
		vcs.LogOptions{Range: "main~2..main", FirstParent: true}))
	if len(ranged) != 2 {
		t.Fatalf("range walk = %d records, want 2 (c5, merge)", len(ranged))
	}
}

func TestLogHistoryPatchBytesCannotSplitRecords(t *testing.T) {
	t.Parallel()

	dir, run, write := newGitRepo(t)
	run("init", "--initial-branch=main")
	write("min.js", "a=1\n")
	run("add", "-A")
	run("commit", "-m", "c1")
	write("min.js", "a=1;b=\x1e2\n")
	write("after.txt", "after\n")
	run("add", "-A")
	run("commit", "-m", "c2")
	write("last.txt", "last\n")
	run("add", "-A")
	run("commit", "-m", "c3")

	records := collect(t, vcs.NewGitOperations().LogHistory(t.Context(), dir, vcs.LogOptions{}))
	if len(records) != 3 || records[1].Subject != "c2" {
		t.Fatalf("got %d records, want 3 with c2 second", len(records))
	}

	c2 := records[1]
	if len(c2.Files) != 2 || c2.Files[0].Path != "after.txt" || c2.Files[1].Path != "min.js" {
		t.Fatalf("c2 files = %+v, want after.txt and min.js", c2.Files)
	}
	minJS := c2.Files[1]
	if !slices.Equal(minJS.Added, []string{"a=1;b=\x1e2"}) || !slices.Equal(minJS.Removed, []string{"a=1"}) {
		t.Fatalf("min.js lines = added %q removed %q", minJS.Added, minJS.Removed)
	}
	if c3 := records[0]; len(c3.Files) != 1 || c3.Files[0].Path != "last.txt" {
		t.Fatalf("c3 files = %+v, want last.txt", c3.Files)
	}
}

func TestTreeAndBlobsAgainstRealGit(t *testing.T) {
	t.Parallel()

	dir := historyRepo(t)
	ops := vcs.NewGitOperations()
	ctx := t.Context()

	entries, err := ops.TreeEntries(ctx, dir, "HEAD")
	if err != nil {
		t.Fatalf("TreeEntries: %v", err)
	}
	paths := make([]string, 0, len(entries))
	shaByPath := map[string]string{}
	for _, e := range entries {
		paths = append(paths, e.Path)
		shaByPath[e.Path] = e.SHA
	}
	slices.Sort(paths)
	want := []string{"bin.dat", "f.txt", "new.txt", "renamed.txt", "side.txt"}
	if !slices.Equal(paths, want) {
		t.Fatalf("tree paths = %v, want %v", paths, want)
	}

	content, err := ops.BlobContent(ctx, dir, shaByPath["f.txt"])
	if err != nil {
		t.Fatalf("BlobContent: %v", err)
	}
	if string(content) != "one\nTWO\nthree\n" {
		t.Fatalf("f.txt = %q", content)
	}
	if got := vcs.BlobSHA(content); got != shaByPath["f.txt"] {
		t.Fatalf("BlobSHA = %q, want %q", got, shaByPath["f.txt"])
	}

	blobs, err := ops.BlobContents(ctx, dir,
		[]string{shaByPath["f.txt"], shaByPath["new.txt"], "0000000000000000000000000000000000000000"})
	if err != nil {
		t.Fatalf("BlobContents: %v", err)
	}
	if len(blobs) != 2 || string(blobs[shaByPath["new.txt"]]) != "new\n" {
		t.Fatalf("BlobContents = %v", blobs)
	}

	// Diff index lines carry abbreviated shas; the result is keyed by the
	// request, not the resolved full sha the batch header echoes back.
	short := shaByPath["f.txt"][:8]
	blobs, err = ops.BlobContents(ctx, dir, []string{short})
	if err != nil {
		t.Fatalf("BlobContents abbreviated: %v", err)
	}
	if string(blobs[short]) != "one\nTWO\nthree\n" {
		t.Fatalf("BlobContents[%q] = %v", short, blobs)
	}
}

func TestDiffRefsAgainstRealGit(t *testing.T) {
	t.Parallel()

	dir := historyRepo(t)
	ops := vcs.NewGitOperations()

	files, err := ops.DiffRefs(t.Context(), dir, "HEAD~3", "HEAD")
	if err != nil {
		t.Fatalf("DiffRefs: %v", err)
	}
	byPath := map[string]vcs.CommitFile{}
	for _, f := range files {
		byPath[f.Path] = f
	}
	if byPath["f.txt"].LinesAdded() != 1 || byPath["f.txt"].LinesDeleted() != 1 {
		t.Fatalf("f.txt diff = %+v", byPath["f.txt"])
	}
	if byPath["side.txt"].Status != vcs.StatusAdded {
		t.Fatalf("side.txt status = %v", byPath["side.txt"].Status)
	}
	if byPath["renamed.txt"].Status != vcs.StatusRenamed ||
		byPath["renamed.txt"].OldPath != "dir with space/s.txt" {
		t.Fatalf("renamed.txt = %+v", byPath["renamed.txt"])
	}
}

func TestDiffWorktreeAgainstRealGit(t *testing.T) {
	t.Parallel()

	dir := historyRepo(t)
	ops := vcs.NewGitOperations()
	ctx := t.Context()

	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("one\nTWO\nthree\nfour\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("u\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	wt, err := ops.DiffWorktree(ctx, dir, "HEAD")
	if err != nil {
		t.Fatalf("DiffWorktree: %v", err)
	}
	if len(wt) != 1 || wt[0].Path != "f.txt" || wt[0].LinesAdded() != 1 {
		t.Fatalf("worktree diff = %+v", wt)
	}

	untracked, err := ops.UntrackedFiles(ctx, dir)
	if err != nil {
		t.Fatalf("UntrackedFiles: %v", err)
	}
	if !slices.Equal(untracked, []string{"untracked.txt"}) {
		t.Fatalf("untracked = %v", untracked)
	}
}

func TestParsePatchHunkStarts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		hunk string
		want vcs.DiffHunk
	}{
		{
			name: "pure insertion after old line 5",
			hunk: "@@ -5,0 +6,2 @@",
			want: vcs.DiffHunk{OldStart: 6, OldLines: 0, NewStart: 6, NewLines: 2},
		},
		{
			name: "pure deletion after new line 2",
			hunk: "@@ -3,2 +2,0 @@",
			want: vcs.DiffHunk{OldStart: 3, OldLines: 2, NewStart: 3, NewLines: 0},
		},
		{
			name: "new file inserts before line 1",
			hunk: "@@ -0,0 +1,3 @@",
			want: vcs.DiffHunk{OldStart: 1, OldLines: 0, NewStart: 1, NewLines: 3},
		},
		{
			name: "single-line change omits counts",
			hunk: "@@ -4 +4 @@",
			want: vcs.DiffHunk{OldStart: 4, OldLines: 1, NewStart: 4, NewLines: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			files := vcs.ParsePatch("diff --git a/f.txt b/f.txt\n" + tt.hunk + "\n")
			if len(files) != 1 || len(files[0].Hunks) != 1 {
				t.Fatalf("ParsePatch = %+v, want one file with one hunk", files)
			}
			if got := files[0].Hunks[0]; got != tt.want {
				t.Fatalf("hunk = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestTreeEntriesSkipsSubmodules(t *testing.T) {
	t.Parallel()

	const (
		blob      = "100644 blob 1111111111111111111111111111111111111111      12\tREADME.md\x00"
		submodule = "160000 commit 2222222222222222222222222222222222222222       -\tvendor/lib\x00"
	)

	tests := []struct {
		name   string
		output string
		want   []vcs.TreeEntry
	}{
		{
			name:   "submodule between blobs is skipped",
			output: blob + submodule + blob,
			want: []vcs.TreeEntry{
				{Path: "README.md", SHA: "1111111111111111111111111111111111111111", Mode: "100644", Size: 12},
				{Path: "README.md", SHA: "1111111111111111111111111111111111111111", Mode: "100644", Size: 12},
			},
		},
		{
			name:   "submodule-only tree is empty",
			output: submodule,
			want:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := vcs.WithCommandRunner(t.Context(),
				func(_ context.Context, _, _ string, _ ...string) (string, error) {
					return tt.output, nil
				})

			got, err := vcs.NewGitOperations().TreeEntries(ctx, "/repo", "HEAD")
			if err != nil {
				t.Fatalf("TreeEntries: %v", err)
			}
			if !slices.Equal(got, tt.want) {
				t.Fatalf("TreeEntries = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParsePatchReadsHunkBodyAsContent(t *testing.T) {
	t.Parallel()

	const header = "diff --git a/lua/init.lua b/lua/init.lua\n" +
		"index 1111111..2222222 100644\n" +
		"--- a/lua/init.lua\n" +
		"+++ b/lua/init.lua\n"

	tests := []struct {
		name        string
		body        string
		wantAdded   []string
		wantRemoved []string
	}{
		{
			name:        "removed Lua comment reads as a header",
			body:        "@@ -3,1 +3,1 @@\n--- note\n+kept\n",
			wantAdded:   []string{"kept"},
			wantRemoved: []string{"-- note"},
		},
		{
			name:        "added line starting with ++ reads as a header",
			body:        "@@ -3,1 +3,1 @@\n-old\n+++ x\n",
			wantAdded:   []string{"++ x"},
			wantRemoved: []string{"old"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			files := vcs.ParsePatch(header + tt.body)
			if len(files) != 1 {
				t.Fatalf("ParsePatch returned %d files, want 1", len(files))
			}
			got := files[0]
			if got.Path != "lua/init.lua" || !slices.Equal(got.Added, tt.wantAdded) ||
				!slices.Equal(got.Removed, tt.wantRemoved) {
				t.Fatalf("file = %+v, want added %v removed %v", got, tt.wantAdded, tt.wantRemoved)
			}
		})
	}
}
