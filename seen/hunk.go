// Package seen records which hunks of a diff a reader has read, and keeps that record on disk
// where two tools, or two copies of one tool, can share it.
//
// A hunk is identified by what it says rather than where it sits: the file it belongs to plus
// every line of the hunk, kinds included, with the line numbers left out. A hunk that shifts down
// the file when something above it grows is the same hunk and stays read; a hunk whose text
// changed is a different one and comes back unread, which is the whole point. Because the id is
// content-addressed and not commit-addressed, a mark survives a force-push or a rebase that leaves
// the hunk's text alone.
//
// The id is sha256 over the path, a NUL, and each line as its kind byte (' ', '+', '-'), its text,
// and a newline, hex encoded. It was second-look's internal/seen and is byte-identical to it, so
// marks that tool wrote before this package existed still read back.
package seen

import (
	"crypto/sha256"
	"encoding/hex"
	"hash"

	"github.com/kyleking/aragonite/diff"
)

// ID is the identity of one hunk, as a hex sha256 digest.
type ID string

// Ref names one hunk of one diff: the file it belongs to, its hunk number as diff.Line.Hunk
// counts it (across the whole diff, from 1), and its id.
type Ref struct {
	Path string
	ID   ID
	Hunk int
}

// Hunk identifies the hunk a diff line belongs to. Lines from one hunk all answer the same ID, so
// marking any line of it marks the hunk. It rescans the diff, so a caller naming every hunk wants
// Hunks instead.
func Hunk(d *diff.Diff, path string, hunk int) ID {
	var buf []byte

	h := sha256.New()
	begin(h, path)

	for i := range d.Files {
		if filePath(&d.Files[i]) != path {
			continue
		}

		for _, l := range d.Files[i].Lines {
			if l.Hunk == hunk {
				buf = line(h, buf, l)
			}
		}
	}

	return ID(hex.EncodeToString(h.Sum(nil)))
}

// Hunks is every hunk in the diff, in patch order, paired with the file it belongs to. A file that
// carries no hunk (a rename, a binary payload, a mode change) has nothing to read and is absent.
//
// The ids are rolled in one pass: a hunk's lines lie together, so hashing each hunk by rescanning
// the diff for its lines would read the diff once per hunk.
func Hunks(d *diff.Diff) []Ref {
	var (
		out []Ref
		buf []byte
	)

	h := sha256.New()

	for i := range d.Files {
		path := filePath(&d.Files[i])
		last := 0

		begin(h, path)

		for _, l := range d.Files[i].Lines {
			if l.Hunk != last {
				if last != 0 {
					out = append(out, Ref{Path: path, Hunk: last, ID: ID(hex.EncodeToString(h.Sum(nil)))})
					begin(h, path)
				}

				last = l.Hunk
			}

			buf = line(h, buf, l)
		}

		if last != 0 {
			out = append(out, Ref{Path: path, Hunk: last, ID: ID(hex.EncodeToString(h.Sum(nil)))})
		}
	}

	return out
}

// Parse is Hunks over git-format diff text, for a caller that holds the patch (`jj diff --git`,
// `git diff`) rather than a parsed diff. Empty text has no hunks.
func Parse(gitDiff []byte) []Ref {
	if len(gitDiff) == 0 {
		return nil
	}

	return Hunks(diff.Parse(gitDiff))
}

func begin(h hash.Hash, path string) {
	h.Reset()
	h.Write([]byte(path))
	h.Write([]byte{0})
}

// line hashes one line as its kind byte, its text, and a newline, through buf so the text is not
// copied to a fresh slice per line. It returns buf for the next line to reuse.
func line(h hash.Hash, buf []byte, l diff.Line) []byte {
	buf = append(buf[:0], l.Kind)
	buf = append(buf, l.Text...)
	buf = append(buf, '\n')
	h.Write(buf)

	return buf
}

// filePath is the name a hunk is filed under: the new path, or the old one for a deletion.
func filePath(f *diff.File) string {
	if f.NewPath != "" {
		return f.NewPath
	}

	return f.OldPath
}
