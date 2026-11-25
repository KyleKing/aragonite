// Package vimtext is a text field with a vim layer, so prose written inside
// these tools (a description, a review comment, a bookmark name) is edited the
// way the people using them edit everything else, without leaving the screen
// it is about.
//
// A [Field] opens in insert mode with the cursor after what it holds, so
// someone who does not use vim just types. The family's text-entry contract
// (docs/keymap.md P14 and P15) holds in every mode: ⏎ saves ([SubmitMsg]),
// ctrl+j and alt+⏎ insert a line break, and esc backs out one level and never
// quits. In insert mode esc goes to normal mode, stepping the cursor back as
// vim does; in visual mode, or with an operator or count half typed, it goes
// back to plain normal mode; and in plain normal mode it cancels the field
// ([CancelMsg], carrying the text so the caller decides whether a draft is
// kept). Nothing leaves the machine on ⏎: saving is the caller's to define.
//
// Normal and visual mode cover what a vim user reaches for in prose: counts;
// h j k l w W b B e E ge gE 0 ^ $ gg G f F t T ; , % { }; the operators d c y
// with motions and doubled, and D C Y (Y is y$, as in nvim: inside the field
// vim's meaning wins over the family's yank); x X s S r ~ J p P; the text
// objects iw aw iW aW i" a" i' a' i` a` i( a( ib i[ a[ i{ a{ iB i< a< ip ap;
// v and V with the operators, o, ~ u U, J, r, and p; u and ctrl+r; . for the
// last change, visual ones included; i a I A o O with counts; and :w, :wq, :x,
// and :q. Insert mode adds ctrl+w, ctrl+u, arrows, home and end, a bracketed
// paste of any length, and ctrl+n and ctrl+p completion from a [Completer].
// The behavior is nvim's with its default options (nostartofline,
// autoindent), and the tests compare it against nvim, or vim set up the same
// way, wherever one is installed.
//
// [WithVim] false turns the modal layer off for a tool or a user that wants
// plain editing; the contract stays, and esc then cancels from insert mode.
//
// The text is edited a grapheme cluster at a time and drawn by display width,
// so a wide character, a combining mark, and an emoji with a skin tone each
// move and delete as one. Content is drawn through glyph.Settle, so a glyph
// terminals measure differently cannot shift a row, while the text itself is
// kept exactly as typed. Lines soft-wrap at a space to the width View is given,
// the field grows from [WithHeight]'s minimum to its maximum and scrolls past
// it, and [WithOneLine] makes a one-row field that scrolls sideways.
//
// The register is one unnamed register, shared between fields with
// [WithRegister] and copied to the system clipboard over OSC 52 with
// [WithClipboard]. $EDITOR stays the way out for anything long: [WithEditor]
// binds ctrl+e to [Edit], which hands the text to the user's editor through
// tea.ExecProcess and answers an [EditedMsg] the field takes back, keeping the
// draft when the editor fails.
//
// A Field draws in three pieces so a screen can frame it its own way: View is
// the text, Footer the mode's badge and keys (or the : line, a refusal from
// [WithValidator], a completion's place in its list), and Render both under
// an optional title. Hints answers the footer's keys for a screen that fits
// them into a footer of its own. [StylesFor] draws it in a skin's faces,
// correct at truecolor, 256 colors, 16 colors, and NO_COLOR.
package vimtext
