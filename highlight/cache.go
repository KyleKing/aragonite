package highlight

import "github.com/kyleking/aragonite/diff"

// Cache holds the spans of one diff's files, each lexed whole on first use.
// It is not safe for concurrent use, and a changed diff needs a new Cache.
type Cache struct {
	d     *diff.Diff
	files map[string]map[diff.LineRef][]Span
}

// NewCache returns an empty Cache over d.
func NewCache(d *diff.Diff) *Cache {
	return &Cache{d: d, files: map[string]map[diff.LineRef][]Span{}}
}

// Spans returns the spans of line l of the file at path, or nil for an unknown file or grammar.
// Each side is lexed apart from the diff's own lines, so state above the first hunk is lost.
func (c *Cache) Spans(path string, l diff.Line) []Span {
	read, ok := c.files[path]
	if !ok {
		read = c.lexFile(path)
		c.files[path] = read
	}

	return read[diff.LineRef{Path: path, Kind: l.Kind, Old: l.Old, New: l.New}]
}

func (c *Cache) lexFile(path string) map[diff.LineRef][]Span {
	var file *diff.File

	for i := range c.d.Files {
		f := &c.d.Files[i]
		if f.NewPath == path || (f.NewPath == "" && f.OldPath == path) {
			file = f

			break
		}
	}

	if file == nil {
		return nil
	}

	var before, after []string

	for _, l := range file.Lines {
		switch l.Kind {
		case diff.KindRemove:
			before = append(before, l.Text)
		case diff.KindAdd:
			after = append(after, l.Text)
		default:
			before = append(before, l.Text)
			after = append(after, l.Text)
		}
	}

	lexedBefore, lexedAfter := Lines(path, before), Lines(path, after)

	out := make(map[diff.LineRef][]Span, len(file.Lines))
	oldAt, newAt := 0, 0

	for _, l := range file.Lines {
		ref := diff.LineRef{Path: path, Kind: l.Kind, Old: l.Old, New: l.New}

		switch l.Kind {
		case diff.KindRemove:
			out[ref] = spanAt(lexedBefore, oldAt)
			oldAt++
		case diff.KindAdd:
			out[ref] = spanAt(lexedAfter, newAt)
			newAt++
		default:
			out[ref] = spanAt(lexedAfter, newAt)
			oldAt++
			newAt++
		}
	}

	return out
}

func spanAt(lexed [][]Span, at int) []Span {
	if at < 0 || at >= len(lexed) {
		return nil
	}

	return lexed[at]
}
