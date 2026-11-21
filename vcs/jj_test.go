package vcs_test

import (
	"strings"
	"testing"

	"github.com/kyleking/aragonite/vcs"
)

func TestParseJJBookmarkList(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected []vcs.JJBookmark
	}{
		{
			name:     "empty",
			input:    "",
			expected: nil,
		},
		{
			name:     "single local bookmark",
			input:    "main\tlocal\n",
			expected: []vcs.JJBookmark{vcs.NewJJBookmark("main", "", 0, 0, "")},
		},
		{
			name:  "bookmark tracked at origin",
			input: "main\tlocal\nmain\torigin\t1\t2\n",
			expected: []vcs.JJBookmark{
				vcs.NewJJBookmark("main", "main@origin", 1, 2, ""),
			},
		},
		{
			name:  "multiple bookmarks",
			input: "main\tlocal\nfeature\tlocal\n",
			expected: []vcs.JJBookmark{
				vcs.NewJJBookmark("main", "", 0, 0, ""),
				vcs.NewJJBookmark("feature", "", 0, 0, ""),
			},
		},
		{
			name: "colocated git remote and other remotes are ignored",
			input: "main\tlocal\n" +
				"main\torigin\t0\t1\n" +
				"main\tgit\t5\t5\n" +
				"main\tfork\t7\t7\n",
			expected: []vcs.JJBookmark{
				vcs.NewJJBookmark("main", "main@origin", 0, 1, ""),
			},
		},
		{
			name:     "a bookmark only on a fork has no upstream",
			input:    "feature\tfork\t3\t4\n",
			expected: []vcs.JJBookmark{vcs.NewJJBookmark("feature", "", 0, 0, "")},
		},
		{
			name:     "local bookmark carries head commit id",
			input:    "main\tlocal\tabc123\n",
			expected: []vcs.JJBookmark{vcs.NewJJBookmark("main", "", 0, 0, "abc123")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			bookmarks := vcs.ParseJJBookmarkList(tt.input)
			if len(bookmarks) != len(tt.expected) {
				t.Fatalf("expected %d bookmarks, got %d", len(tt.expected), len(bookmarks))
			}
			for i, expected := range tt.expected {
				if bookmarks[i] != expected {
					t.Errorf("bookmark %d: expected %+v, got %+v", i, expected, bookmarks[i])
				}
			}
		})
	}
}

func TestParseJJStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "empty status",
			input:    "",
			expected: 0,
		},
		{
			name:     "working copy clean",
			input:    "Working copy : abc123\nParent commit: def456",
			expected: 0,
		},
		{
			name:     "added file",
			input:    "A file.txt\nWorking copy changes:",
			expected: 1,
		},
		{
			name:     "modified file",
			input:    "M file.txt",
			expected: 1,
		},
		{
			name:     "deleted file",
			input:    "D file.txt",
			expected: 1,
		},
		{
			name:     "renamed file",
			input:    "R old.txt -> new.txt",
			expected: 1,
		},
		{
			name:     "multiple changes",
			input:    "A new.txt\nM changed.txt\nD removed.txt",
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := parseJJStatusCounts(tt.input)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func parseJJStatusCounts(out string) int {
	count := 0
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "A ") || strings.HasPrefix(trimmed, "M ") ||
			strings.HasPrefix(trimmed, "D ") || strings.HasPrefix(trimmed, "R ") {
			count++
		}
	}

	return count
}

func TestOperationsVCSType(t *testing.T) {
	t.Parallel()

	if got := vcs.NewJJOperations().VCSType(); got != vcs.TypeJJ || got.String() != "jj" {
		t.Errorf("jj operations report %v (%s)", got, got.String())
	}

	if got := vcs.NewGitOperations().VCSType(); got != vcs.TypeGit || got.String() != "git" {
		t.Errorf("git operations report %v (%s)", got, got.String())
	}
}
