package highlight

import (
	"sync"

	"github.com/kyleking/aragonite/diff"
)

// Cache holds the spans of one diff's files, each lexed whole once. It is safe for concurrent
// use, so a caller can lex a file in the background with Start and draw from Peek until the
// spans land, rather than blocking a frame on the lexer. A changed diff needs a new Cache.
type Cache struct {
	d     *diff.Diff
	files map[string]*lexed
	mu    sync.Mutex
}

// A lexed is one file's spans, filled once by whichever caller lexes it first. Done closes
// when spans is set, and spans is never written after.
type lexed struct {
	spans map[diff.LineRef][]Span
	done  chan struct{}
}

// NewCache returns an empty Cache over d.
func NewCache(d *diff.Diff) *Cache {
	return &Cache{d: d, files: map[string]*lexed{}}
}

// Spans returns the spans of line l of the file at path, or nil for an unknown file or grammar.
// Each side is lexed apart from the diff's own lines, so state above the first hunk is lost.
// It lexes the file on first use and blocks until that lexing is done, including a lexing
// Start already began.
func (c *Cache) Spans(path string, l diff.Line) []Span {
	f, owner := c.claim(path)
	if owner {
		c.fill(path, f)
	}

	<-f.done

	return f.spans[diff.LineRef{Path: path, Kind: l.Kind, Old: l.Old, New: l.New}]
}

// Start lexes the file at path on another goroutine, unless it is lexed or being lexed already,
// and returns a channel closed once its spans are ready. Waiting on it is how a caller learns
// when to redraw with color.
func (c *Cache) Start(path string) <-chan struct{} {
	f, owner := c.claim(path)
	if owner {
		go c.fill(path, f)
	}

	return f.done
}

// Peek returns the spans of line l without ever lexing. Ready is false while the file has not
// been lexed, and the caller draws the line plain until it is; once ready, nil spans mean the
// line has no color, as from Spans.
func (c *Cache) Peek(path string, l diff.Line) ([]Span, bool) {
	c.mu.Lock()
	f := c.files[path]
	c.mu.Unlock()

	if f == nil {
		return nil, false
	}

	select {
	case <-f.done:
		return f.spans[diff.LineRef{Path: path, Kind: l.Kind, Old: l.Old, New: l.New}], true
	default:
		return nil, false
	}
}

// claim returns the entry for path, and whether this caller created it and so must fill it.
func (c *Cache) claim(path string) (*lexed, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if f, ok := c.files[path]; ok {
		return f, false
	}

	f := &lexed{done: make(chan struct{})}
	c.files[path] = f

	return f, true
}

func (c *Cache) fill(path string, f *lexed) {
	defer close(f.done)

	f.spans = c.lexFile(path)
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

	lexedBefore, lexedAfter := bothSides(path, before, after)

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

// bothSides lexes the old side on another goroutine while this one lexes the new, since the
// two are independent and the lexer is most of what opening a file costs. A side with no
// lines is skipped rather than lexed empty.
func bothSides(path string, before, after []string) ([][]Span, [][]Span) {
	if !hasChange(before, after) {
		lexed := Lines(path, after)

		return lexed, lexed
	}

	var (
		wg                      sync.WaitGroup
		lexedBefore, lexedAfter [][]Span
	)

	if len(before) > 0 {
		wg.Go(func() { lexedBefore = Lines(path, before) })
	}

	if len(after) > 0 {
		lexedAfter = Lines(path, after)
	}

	wg.Wait()

	return lexedBefore, lexedAfter
}

// hasChange is false when both sides hold the same lines, which a diff of only context does.
func hasChange(before, after []string) bool {
	if len(before) != len(after) {
		return true
	}

	for i := range before {
		if before[i] != after[i] {
			return true
		}
	}

	return false
}

func spanAt(lexed [][]Span, at int) []Span {
	if at < 0 || at >= len(lexed) {
		return nil
	}

	return lexed[at]
}
