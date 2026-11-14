package diff_test

import (
	"testing"

	"github.com/kyleking/aragonite/diff"
)

func TestMechanicalHunks(t *testing.T) {
	t.Parallel()

	patch := "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n" +
		"@@ -1,2 +1,2 @@\n-\tfoo()\n-\tbar()\n+\t\tfoo()\n+\t\tbar()\n" +
		"@@ -10,2 +10,2 @@\n-a()\n-b()\n+b()\n+a()\n" +
		"@@ -20,2 +20,2 @@\n-\tfoo()\n-x := 1\n+\t\tfoo()\n+x := 2\n" +
		"diff --git a/b.go b/b.go\n--- a/b.go\n+++ b/b.go\n" +
		"@@ -1,1 +1,1 @@\n-v := 1\n+v := 2\n"

	got := diff.Parse([]byte(patch)).MechanicalHunks()

	want := map[diff.HunkRef]string{
		{Path: "a.go", Hunk: 1}: "reindent only",
		{Path: "a.go", Hunk: 2}: "whitespace only",
	}

	if len(got) != len(want) {
		t.Errorf("MechanicalHunks = %v, want %v", got, want)
	}

	for at, why := range want {
		if got[at] != why {
			t.Errorf("hunk %v folded as %q, want %q", at, got[at], why)
		}
	}
}
