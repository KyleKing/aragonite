// Package filter narrows a list to the rows worth looking at, the shape every
// tool here reaches for once a list is long enough to need a live search: a
// predicate to combine and apply, a smart-case substring match for the base
// case, and a scoped-prefix query so "kind:thread pool" or "r:foo b:bar rest"
// parses the same way wherever the prefixes differ.
//
// What stays local to each caller is which prefixes it recognizes and what
// they mean: a "kind:" that matches a row's own field, or a "p:" that looks up
// a pull request, is domain knowledge this package has no way to share.
package filter

import "strings"

// Predicate reports whether one item should be kept.
type Predicate[T any] func(T) bool

// All is the predicate that keeps an item only when every one of preds does,
// short-circuiting scans left to right. An empty preds keeps everything.
func All[T any](preds ...Predicate[T]) Predicate[T] {
	return func(v T) bool {
		for _, p := range preds {
			if !p(v) {
				return false
			}
		}

		return true
	}
}

// Any is the predicate that keeps an item when at least one of preds does.
// An empty preds keeps nothing.
func Any[T any](preds ...Predicate[T]) Predicate[T] {
	return func(v T) bool {
		for _, p := range preds {
			if p(v) {
				return true
			}
		}

		return false
	}
}

// Not inverts a predicate, which is how a caller's "not X" toggle composes
// with All/Any instead of needing its own inverted copy of every mode.
func Not[T any](p Predicate[T]) Predicate[T] {
	return func(v T) bool { return !p(v) }
}

// Keep returns the items of in that p keeps, preserving their order.
func Keep[T any](in []T, p Predicate[T]) []T {
	out := make([]T, 0, len(in))

	for _, v := range in {
		if p(v) {
			out = append(out, v)
		}
	}

	return out
}

// Match reports whether text contains pattern, case-insensitively unless
// pattern itself carries an uppercase letter. That is the rule every editor's
// search box uses and the one nobody has to be told, so it is the default a
// caller reaches for before reaching past it to a fuzzy or glob matcher of
// its own.
func Match(pattern, text string) bool {
	if pattern == strings.ToLower(pattern) {
		return strings.Contains(strings.ToLower(text), pattern)
	}

	return strings.Contains(text, pattern)
}

// Tokens splits query into the plain text left over and the scoped words it
// carries, a word being scoped when it starts with one of scopes followed by
// ":" (matched case-insensitively, e.g. "kind:" catches "Kind:thread" too).
// "kind:thread pool" with scopes "kind" gives back text "pool" and
// scoped["kind"] = ["thread"]; an unrecognized prefix like "p:123" against
// scopes "kind" is left in text untouched, since only a caller knows which
// prefixes it means to parse.
func Tokens(query string, scopes ...string) (string, map[string][]string) {
	scoped := make(map[string][]string, len(scopes))

	var plain []string

	for _, word := range strings.Fields(query) {
		matched := false

		for _, scope := range scopes {
			if rest, ok := cutFold(word, scope+":"); ok {
				scoped[scope] = append(scoped[scope], rest)
				matched = true

				break
			}
		}

		if !matched {
			plain = append(plain, word)
		}
	}

	return strings.Join(plain, " "), scoped
}

// cutFold reports whether word starts with prefix, matched case-insensitively,
// and gives back what follows it.
func cutFold(word, prefix string) (string, bool) {
	if len(word) < len(prefix) || !strings.EqualFold(word[:len(prefix)], prefix) {
		return "", false
	}

	return word[len(prefix):], true
}
