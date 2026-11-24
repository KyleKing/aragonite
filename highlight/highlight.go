// Package highlight lexes source into the few classes a diff is worth coloring by.
//
// It answers in byte ranges rather than escape sequences, so the caller keeps every
// decision about color and can lay a diff's own background band and word marks under it.
package highlight

import (
	"path"
	"strings"
	"sync"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// Class is what a run of source is: a handful of cases rather than chroma's several
// hundred.
type Class uint8

// The classes, which is every distinction a caller can draw a color for.
const (
	// Plain is everything a grammar has nothing particular to say about.
	Plain Class = iota
	Comment
	Keyword
	String
	Number
	Name
	Function
	Type
	Punctuation
	// Operator is the symbol kind a keyword is not: +, :=, in as a word.
	Operator
	// Field is a name a receiver owns: attributes, properties, decorators, and labels.
	Field
	// Constant is a name that is a value: true, nil, constants a lexer knows.
	Constant
	// Namespace is the name of the package a name comes from.
	Namespace
	// Escape is the part of a string that is not its text: escapes, regexes,
	// interpolation, and the affixes that mark them.
	Escape
)

// Span is a run of one line under one class. Spans of a line are in order and do not
// overlap, and a byte no span covers is Plain.
type Span struct {
	From  int
	To    int
	Class Class
}

// Lines lexes src as one unit, so multi-line strings and comments carry across lines.
// The grammar comes from the file name alone, and a name no grammar answers for yields nil.
func Lines(name string, src []string) [][]Span {
	return lex(matchName(path.Base(name)), src)
}

// Tagged lexes src under the grammar a fenced block's language tag names, and yields nil
// where the tag is empty or names none.
func Tagged(lang string, src []string) [][]Span {
	if lang == "" {
		return nil
	}

	return lex(matchTag(lang), src)
}

// Resolving a grammar walks every registered lexer's file globs, which costs milliseconds a
// call, so each answer is kept for the life of the process. A name no grammar answers for is
// kept too, as a grammar with no lexer, since asking again would walk the globs again.
var (
	byName sync.Map // base file name -> *grammar
	byTag  sync.Map // fence language tag -> *grammar
)

// grammar is a resolved lexer, held by pointer so a cache answers a concrete type.
type grammar struct {
	lexer chroma.Lexer
}

func matchName(base string) *grammar {
	return resolved(&byName, base, lexers.Match)
}

func matchTag(lang string) *grammar {
	return resolved(&byTag, lang, lexers.Get)
}

func resolved(seen *sync.Map, key string, find func(string) chroma.Lexer) *grammar {
	if got, ok := seen.Load(key); ok {
		if g, isGrammar := got.(*grammar); isGrammar {
			return g
		}
	}

	g := &grammar{lexer: find(key)}
	seen.Store(key, g)

	return g
}

func lex(g *grammar, src []string) [][]Span {
	if g.lexer == nil {
		return nil
	}

	lexer := chroma.Coalesce(g.lexer)

	joined := strings.Join(src, "\n")

	it, err := lexer.Tokenise(nil, joined)
	if err != nil {
		return nil
	}

	out := make([][]Span, len(src))
	at, line := 0, 0

	for _, tok := range it.Tokens() {
		class := classOf(tok.Type)

		for _, part := range strings.SplitAfter(tok.Value, "\n") {
			if part == "" {
				continue
			}

			text := strings.TrimSuffix(part, "\n")

			if line < len(out) && text != "" && class != Plain {
				out[line] = append(out[line], Span{From: at, To: at + len(text), Class: class})
			}

			if strings.HasSuffix(part, "\n") {
				at, line = 0, line+1

				continue
			}

			at += len(text)
		}
	}

	return out
}

// The specific Name kinds are tested before the Name category, or a field would answer
// as an ordinary name. Number is tested before Literal, or it would answer as a string.
func classOf(t chroma.TokenType) Class {
	switch {
	case t.InCategory(chroma.Comment):
		return Comment
	case t == chroma.KeywordType, t == chroma.NameClass,
		t == chroma.NameBuiltin, t == chroma.NameBuiltinPseudo:
		return Type
	case t.InSubCategory(chroma.NameFunction):
		return Function
	case t.InCategory(chroma.Operator):
		return Operator
	case t == chroma.NameConstant, t == chroma.KeywordConstant,
		t == chroma.LiteralStringBoolean, t == chroma.NameVariableMagic:
		return Constant
	case t == chroma.NameNamespace:
		return Namespace
	case t == chroma.NameAttribute, t == chroma.NameDecorator,
		t == chroma.NameProperty, t == chroma.NameVariableInstance,
		t == chroma.NameLabel:
		return Field
	case t == chroma.LiteralStringEscape, t == chroma.LiteralStringRegex,
		t == chroma.LiteralStringInterpol, t == chroma.LiteralStringAffix:
		return Escape
	case t.InCategory(chroma.Keyword):
		return Keyword
	case t.InSubCategory(chroma.LiteralNumber):
		return Number
	case t.InCategory(chroma.Literal):
		return String
	case t.InCategory(chroma.Name):
		return Name
	case t.InCategory(chroma.Punctuation):
		return Punctuation
	}

	return Plain
}
