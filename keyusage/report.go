package keyusage

import (
	"cmp"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
	"text/tabwriter"
	"time"
)

// Row is one binding, or one missed key, and how it has been used.
type Row struct {
	Stat
	Ctx  string
	Path string
	What string
}

// Cold lists every binding in km coldest first: by count, then by last use.
// A prefix key is listed through its chords rather than on its own.
func (u Usage) Cold(km Keymap) []Row {
	var rows []Row

	for ctx, bs := range km {
		rows = u.walk(rows, ctx, "", bs)
	}

	slices.SortFunc(rows, func(a, b Row) int {
		return cmp.Or(
			cmp.Compare(a.Count, b.Count),
			a.Last.Compare(b.Last),
			cmp.Compare(a.Ctx, b.Ctx),
			cmp.Compare(a.Path, b.Path),
		)
	})

	return rows
}

func (u Usage) walk(rows []Row, ctx, prefix string, bs []Binding) []Row {
	for _, b := range bs {
		if len(b.Keys) == 0 || allIgnored(b.Keys) {
			continue
		}

		path := strings.TrimSpace(prefix + " " + b.Keys[0])
		if len(b.Kids) > 0 {
			rows = u.walk(rows, ctx, path, b.Kids)

			continue
		}

		rows = append(rows, Row{Ctx: ctx, Path: path, What: b.What, Stat: u.Stat(ctx, path)})
	}

	return rows
}

// Misses lists keys pressed where no binding answers them, most frequent
// first. A chord's miss keeps its prefix ("a q"), which is how a reflex aimed
// at the wrong second key shows up.
func (u Usage) Misses() []Row {
	var rows []Row

	for ctx, paths := range u.misses {
		for path, s := range paths {
			rows = append(rows, Row{Ctx: ctx, Path: path, Stat: s})
		}
	}

	slices.SortFunc(rows, func(a, b Row) int {
		return cmp.Or(cmp.Compare(b.Count, a.Count), cmp.Compare(a.Ctx, b.Ctx), cmp.Compare(a.Path, b.Path))
	})

	return rows
}

// Overload is one key that does different things in different contexts.
type Overload struct {
	Key  string
	Uses []Row
}

// Overloads lists the top-level keys bound in two or more contexts to
// different things, each with how it was used there, busiest key first. A key
// whose meaning shifts with the screen is the one muscle memory carries into
// the wrong place.
func (u Usage) Overloads(km Keymap) []Overload {
	byKey := map[string][]Row{}

	for ctx, bs := range km {
		for _, b := range bs {
			for _, k := range b.Keys {
				if ignored(k) {
					continue
				}

				byKey[k] = append(byKey[k], Row{Ctx: ctx, Path: b.Keys[0], What: b.What, Stat: u.Stat(ctx, b.Keys[0])})
			}
		}
	}

	var out []Overload

	for _, k := range slices.Sorted(maps.Keys(byKey)) {
		uses := byKey[k]
		if !differs(uses) {
			continue
		}

		slices.SortFunc(uses, func(a, b Row) int { return cmp.Compare(a.Ctx, b.Ctx) })
		out = append(out, Overload{Key: k, Uses: uses})
	}

	slices.SortStableFunc(out, func(a, b Overload) int { return cmp.Compare(total(b.Uses), total(a.Uses)) })

	return out
}

// merge folds never-used rows that share a path and a meaning into one row
// naming every context, since screens built from one keymap repeat each other.
func merge(rows []Row) []Row {
	type id struct{ path, what string }

	ctxs := map[id][]string{}

	var order []id

	for _, r := range rows {
		k := id{r.Path, r.What}
		if _, ok := ctxs[k]; !ok {
			order = append(order, k)
		}

		ctxs[k] = append(ctxs[k], r.Ctx)
	}

	out := make([]Row, 0, len(order))
	for _, k := range order {
		out = append(out, Row{Ctx: strings.Join(ctxs[k], ","), Path: k.path, What: k.what})
	}

	slices.SortFunc(out, func(a, b Row) int { return cmp.Or(cmp.Compare(a.Ctx, b.Ctx), cmp.Compare(a.Path, b.Path)) })

	return out
}

func allIgnored(keys []string) bool {
	for _, k := range keys {
		if !ignored(k) {
			return false
		}
	}

	return true
}

func differs(uses []Row) bool {
	for _, r := range uses[1:] {
		if r.What != uses[0].What {
			return true
		}
	}

	return false
}

func total(rows []Row) int {
	n := 0
	for _, r := range rows {
		n += r.Count
	}

	return n
}

// Write prints the report a tool's `keys` subcommand shows: bindings never
// used, bindings gone cold since stale, keys that missed, and keys whose
// meaning changes with the context.
func Write(w io.Writer, u Usage, km Keymap, stale time.Time) error {
	tw := tabwriter.NewWriter(w, 0, 0, colGap, ' ', 0)
	p := &printer{w: tw}

	var never, cold []Row

	for _, r := range u.Cold(km) {
		switch {
		case r.Count == 0:
			never = append(never, r)
		case r.Last.Before(stale):
			cold = append(cold, r)
		}
	}

	never = merge(never)
	p.section(fmt.Sprintf("never used (%d)", len(never)))

	for _, r := range never {
		p.f("  %s\t%s\t%s\n", r.Ctx, r.Path, r.What)
	}

	p.section(fmt.Sprintf("not used since %s (%d)", stale.Format(time.DateOnly), len(cold)))

	for _, r := range cold {
		p.f("  %s\t%s\t%d\t%s\t%s\n", r.Ctx, r.Path, r.Count, r.Last.Format(time.DateOnly), r.What)
	}

	misses := u.Misses()
	p.section(fmt.Sprintf("pressed with nothing bound (%d)", len(misses)))

	for _, r := range misses {
		p.f("  %s\t%s\t%d\n", r.Ctx, r.Path, r.Count)
	}

	p.refusals(u.Refusals())

	overloads := u.Overloads(km)
	p.section(fmt.Sprintf("one key, different meanings (%d)", len(overloads)))

	for _, o := range overloads {
		for i, r := range o.Uses {
			key := ""
			if i == 0 {
				key = o.Key
			}

			p.f("  %s\t%s\t%d\t%s\n", key, r.Ctx, r.Count, r.What)
		}
	}

	if n := len(u.Unmapped); n > 0 {
		p.section(fmt.Sprintf("contexts with no keymap (%d)", n))

		for _, ctx := range slices.Sorted(maps.Keys(u.Unmapped)) {
			p.f("  %s\t%d\n", ctx, u.Unmapped[ctx])
		}
	}

	if u.Skipped > 0 {
		p.f("\n%d unreadable lines skipped\n", u.Skipped)
	}

	if p.err != nil {
		return fmt.Errorf("writing the key report: %w", p.err)
	}

	if err := tw.Flush(); err != nil {
		return fmt.Errorf("writing the key report: %w", err)
	}

	return nil
}

const colGap = 2

// printer keeps the first write error so the report reads as straight-line
// prints.
type printer struct {
	w   io.Writer
	err error
}

func (p *printer) f(format string, args ...any) {
	if p.err == nil {
		_, p.err = fmt.Fprintf(p.w, format, args...)
	}
}

func (p *printer) section(title string) { p.f("\n%s\n", title) }

// refusals lists the actions pressed where they could not run, and nothing when there are none,
// so a log from before gating was recorded reads as it always did.
func (p *printer) refusals(rows []Row) {
	if len(rows) == 0 {
		return
	}

	p.section(fmt.Sprintf("pressed where it could not run (%d)", len(rows)))

	for _, r := range rows {
		p.f("  %s\t%s\t%d\n", r.Ctx, r.Path, r.Count)
	}
}
