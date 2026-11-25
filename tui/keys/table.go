package keys

import "slices"

// The legend groups the shared bindings are filed under. A tool may file its own bindings under
// these or under groups of its own.
const (
	GroupGeneral = "General"
	GroupMove    = "Move"
	GroupMotion  = "Jump"
	GroupFind    = "Find"
	GroupRead    = "Read"
	GroupFold    = "Fold"
	GroupFrame   = "Frame"
	GroupView    = "View"
)

// Action names a tool dispatches on. Each is a compatibility promise: a user's config rebinds by
// these names, so one is renamed only with a deprecation alias for a release.
const (
	ActionHelp        = "help"
	ActionPalette     = "palette"
	ActionSearch      = "search"
	ActionAgain       = "again"
	ActionReverse     = "reverse"
	ActionQuit        = "quit"
	ActionBack        = "back"
	ActionCancel      = "cancel"
	ActionUndo        = "undo"
	ActionRedo        = "redo"
	ActionReload      = "reload"
	ActionJumpBack    = "jump-back"
	ActionJumpForward = "jump-forward"
	ActionSwitchPane  = "switch-pane"
	ActionRepeat      = "repeat"
	ActionShell       = "shell"
	ActionInspect     = "inspect"
	ActionYank        = "yank"
	ActionLabelJump   = "label-jump"
	ActionTop         = "top"
	ActionBottom      = "bottom"
	ActionOpenFile    = "open-file"
	ActionDefinition  = "definition"
	ActionReferences  = "references"
	ActionCalls       = "calls"
	ActionMessages    = "messages"

	ActionLineDown     = "line-down"
	ActionLineUp       = "line-up"
	ActionHalfPageDown = "half-page-down"
	ActionHalfPageUp   = "half-page-up"
	ActionPageDown     = "page-down"
	ActionPageUp       = "page-up"
	ActionScrollDown   = "scroll-down"
	ActionScrollUp     = "scroll-up"
	ActionSideOld      = "side-old"
	ActionSideNew      = "side-new"
	ActionFilter       = "filter"
	ActionRead         = "read"
	ActionOnlyUnread   = "only-unread"
	ActionRounds       = "rounds"
	ActionOutline      = "outline"
	ActionVisual       = "visual"
	ActionWhitespace   = "fold-whitespace"
	ActionCosmetic     = "fold-cosmetic"
	ActionMoreContext  = "context-more"
	ActionLessContext  = "context-less"

	ActionFoldToggle  = "fold-toggle"
	ActionFoldInvert  = "fold-invert"
	ActionFoldOpenAll = "fold-open-all"
	ActionFoldAll     = "fold-all"
	ActionFrameMiddle = "frame-middle"
	ActionFrameTop    = "frame-top"
	ActionFrameBottom = "frame-bottom"

	ActionViewSideBySide = "view-sbs"
	ActionViewWrap       = "view-wrap"
	ActionViewNumbers    = "view-numbers"
	ActionViewRefine     = "view-refine"
	ActionViewGrammar    = "view-grammar"
	ActionViewImages     = "view-images"
	ActionViewCycle      = "view-cycle"

	ActionColumnLeft     = "column-left"
	ActionColumnRight    = "column-right"
	ActionMoveLeft       = "move-left"
	ActionMoveRight      = "move-right"
	ActionSendTo         = "send-to"
	ActionDescribe       = "describe"
	ActionDescribeEditor = "describe-editor"

	ActionTakeSide    = "take-side"
	ActionTakeBase    = "take-base"
	ActionTakeAll     = "take-all"
	ActionEditRegion  = "edit-region"
	ActionResetRegion = "reset-region"
	ActionMergeTool   = "merge-tool"

	// The family verbs on reserved keys, which only some tools have (Verbs).
	ActionInsert       = "insert"
	ActionPublish      = "publish"
	ActionBranches     = "branches"
	ActionBranchDelete = "branch-delete"
	ActionInfo         = "info"
)

// Groups the split columns and the conflict viewer file their bindings under.
const (
	GroupColumns = "Columns"
	GroupResolve = "Resolve"
)

// Digits is "1" to "9", the keys that name a column or a conflict side by its number.
func Digits() []string {
	return []string{"1", "2", "3", "4", "5", "6", "7", "8", "9"}
}

// Global is what every screen of every tool offers unless the tool leaves a binding out. The
// motion repeats on n and N apply everywhere but jojo's log, where those keys are verbs.
func Global() []Binding {
	g := func(action, keys, what, short string) Binding {
		return Binding{Action: action, Keys: []string{keys}, What: what, Short: short, Group: GroupGeneral}
	}

	out := []Binding{
		g(ActionHelp, "?", "help", ""),
		g(ActionPalette, ":", "command palette", "commands"),
		g(ActionQuit, "q", "close, or quit at the root", "quit"),
		g(ActionBack, "esc", "back out one level", "back"),
		g(ActionCancel, "ctrl+c", "cancel the running command, else quit", "cancel"),
		g(ActionUndo, "u", "undo", ""),
		g(ActionRedo, "ctrl+r", "redo", ""),
		g(ActionReload, "ctrl+l", "reload from the source", "reload"),
		g(ActionSwitchPane, "tab", "next pane", "pane"),
		g(ActionRepeat, ".", "repeat the last change", "repeat"),
		g(ActionShell, "!", "shell in the repository root", "shell"),
		g(ActionInspect, "K", "what is this", "inspect"),
		g(ActionYank, "Y", "yank a reference", "yank"),
		g(ActionLabelJump, "'", "jump to a labeled row", "jump"),
		g(ActionJumpBack, "ctrl+o", "jump list back", "back"),
		g(ActionJumpForward, "ctrl+i", "jump list forward", "forward"),
		{Action: ActionTop, Keys: []string{"g g"}, What: "top", Group: GroupMove},
		{Action: ActionBottom, Keys: []string{"G"}, What: "bottom", Group: GroupMove},
		{Action: ActionOpenFile, Keys: []string{"g f"}, What: "open the file at the line", Group: GroupMove},
		{Action: ActionDefinition, Keys: []string{"g d"}, What: "definition", Group: GroupMove},
		{Action: ActionReferences, Keys: []string{"g r"}, What: "references", Group: GroupMove},
		{Action: ActionCalls, Keys: []string{"g c"}, What: "call graph", Group: GroupMove},
		{Action: ActionMessages, Keys: []string{"g m"}, What: "messages and the last failure", Group: GroupGeneral},
		{Action: ActionSearch, Keys: []string{"/"}, What: "search", Group: GroupFind},
		{
			Action: ActionAgain, Keys: []string{"n"}, What: "repeat the last motion", Short: "next",
			Group: GroupMotion, Contexts: ^Log,
		},
		{
			Action: ActionReverse, Keys: []string{"N"}, What: "repeat the last motion backward", Short: "prev",
			Group: GroupMotion, Contexts: ^Log,
		},
	}

	return out
}

// DiffView is the diff-viewing vocabulary: moving (which applies in lists and pickers too), the ]
// and [ motions over every object, finding, read marks, folds, framing, and the view toggles. Tools
// drop what they lack with Without.
func DiffView() []Binding {
	d := func(action, group, what, short string, keys ...string) Binding {
		return Binding{Action: action, Keys: keys, What: what, Short: short, Group: group, Contexts: Diff}
	}

	// Moving applies wherever there is something to move through: a diff, a list, a picker.
	move := func(action, what, short string, keys ...string) Binding {
		return Binding{Action: action, Keys: keys, What: what, Short: short, Group: GroupMove}
	}

	out := []Binding{
		move(ActionLineDown, "down a line", "", "j", "down"),
		move(ActionLineUp, "up a line", "", "k", "up"),
		move(ActionHalfPageDown, "half a page down", "page", "ctrl+d"),
		move(ActionHalfPageUp, "half a page up", "", "ctrl+u"),
		move(ActionPageDown, "a page down", "", "ctrl+f"),
		move(ActionPageUp, "a page up", "", "ctrl+b"),
		move(ActionScrollDown, "scroll a line down", "", "ctrl+e", "shift+down"),
		move(ActionScrollUp, "scroll a line up", "", "ctrl+y", "shift+up"),
		// In the split columns h and l move between columns, so the sides are every other diff.
		{Action: ActionSideOld, Keys: []string{"h"}, What: "old side", Group: GroupMove, Contexts: Diff &^ Split},
		{Action: ActionSideNew, Keys: []string{"l"}, What: "new side", Group: GroupMove, Contexts: Diff &^ Split},
		{
			Action: ActionFilter, Keys: []string{"f"}, What: "show only files matching a pattern", Short: "filter",
			Group: GroupFind, Contexts: Diff | List,
		},
		Tick(TickRead),
		d(ActionOnlyUnread, GroupRead, "only what is unread", "unread", "U"),
		d(ActionRounds, GroupRead, "choose the round to compare against", "rounds", "H"),
		d(ActionOutline, GroupRead, "outline", "", "I"),
		d(ActionVisual, GroupRead, "select a range of lines", "visual", "v", "V"),
		d(ActionWhitespace, GroupFold, "fold whitespace-only changes", "ws", "w"),
		d(ActionCosmetic, GroupFold, "fold changes with no code effect", "cosmetic", "W"),
		d(ActionMoreContext, GroupFold, "more context", "", "+"),
		// - is the flag prefix where a mode takes flags, which the split columns do (-p).
		{
			Action: ActionLessContext, Keys: []string{"-"}, What: "less context", Group: GroupFold,
			Contexts: Diff &^ Split,
		},
	}

	out = append(out, Motions(Objects())...)
	out = append(out, Folds()...)

	return append(out, ViewOptions()...)
}

// Folds is the z page: folding and where the cursor line sits on screen.
func Folds() []Binding {
	z := func(action, key, group, what string) Binding {
		return Binding{Action: action, Keys: []string{"z " + key}, What: what, Group: group, Contexts: Diff}
	}

	return []Binding{
		z(ActionFoldToggle, "a", GroupFold, "fold or unfold this"),
		z(ActionFoldInvert, "i", GroupFold, "invert every fold"),
		z(ActionFoldOpenAll, "R", GroupFold, "open every fold"),
		z(ActionFoldAll, "M", GroupFold, "fold everything"),
		z(ActionFrameMiddle, "z", GroupFrame, "line to the middle"),
		z(ActionFrameTop, "t", GroupFrame, "line to the top"),
		z(ActionFrameBottom, "b", GroupFrame, "line to the bottom"),
	}
}

// ViewOptions is the \ page: how a diff is drawn.
func ViewOptions() []Binding {
	v := func(action, key, what, short string) Binding {
		return Binding{
			Action: action, Keys: []string{`\ ` + key}, What: what, Short: short, Group: GroupView, Contexts: Diff,
		}
	}

	return []Binding{
		v(ActionViewSideBySide, "s", "side by side", "sbs"),
		v(ActionViewWrap, "w", "wrap long lines", "wrap"),
		v(ActionViewNumbers, "n", "line numbers", "numbers"),
		v(ActionViewRefine, "r", "word-level refinement", "words"),
		v(ActionViewGrammar, "g", "syntax colors", "grammar"),
		v(ActionViewImages, "i", "images", "images"),
		v(ActionViewCycle, `\`, "next view preset", "preset"),
	}
}

// SplitColumns is the split columns mode: one column per commit the split makes, the selection moved
// between them, and each described. Moving within a column, selecting, and folding are the diff's
// own keys, and -p is FlagBindings(VerbSplit).
func SplitColumns() []Binding {
	c := func(action, what, short string, keys ...string) Binding {
		return Binding{Action: action, Keys: keys, What: what, Short: short, Group: GroupColumns, Contexts: Split}
	}

	return []Binding{
		c(ActionColumnLeft, "focus the column to the left", "column", "h"),
		c(ActionColumnRight, "focus the column to the right", "", "l"),
		c(ActionMoveLeft, "move the selection one column left", "move", "<"),
		c(ActionMoveRight, "move the selection one column right, or to a new one", "", ">"),
		c(ActionSendTo, "send the selection to column N", "send", Digits()...),
		c(ActionDescribe, "describe the focused column", "describe", "d"),
		c(ActionDescribeEditor, "describe the focused column in $EDITOR", "", "D"),
	}
}

// ConflictPicks is the conflict viewer's choices for the region at point. Stepping between regions
// is the conflict object (]x, [x) and files the file object, so neither costs a key here.
func ConflictPicks() []Binding {
	r := func(action, what, short string, keys ...string) Binding {
		return Binding{Action: action, Keys: keys, What: what, Short: short, Group: GroupResolve, Contexts: Conflict}
	}

	return []Binding{
		r(ActionTakeSide, "take side N", "side", Digits()...),
		r(ActionTakeBase, "take the base", "base", "0"),
		r(ActionTakeAll, "take every side in order", "all", "a"),
		r(ActionEditRegion, "edit the region in $EDITOR", "edit", "e"),
		r(ActionResetRegion, "put the region back to conflicted", "reset", "R"),
		r(ActionMergeTool, "hand the file to the configured merge tool", "merge tool", "M"),
	}
}

// Groups the family verbs file under.
const (
	GroupWrite  = "Write"
	GroupBranch = "Branch"
)

// Verbs is the family's verbs on reserved keys that not every tool has: i writes prose at point
// (a comment on a line, a reply on a thread, a comment on the pull request, the severity chord
// following where a tool has one), P opens the publish page, "b b" and "b d" are the branches, and
// "g i" is the card about what is open. A tool merges the ones it has; a tool without one leaves the key
// unbound rather than reuse it, which Unreserved checks.
func Verbs() []Binding {
	v := func(action, group, what, short, keys string) Binding {
		return Binding{Action: action, Keys: []string{keys}, What: what, Short: short, Group: group}
	}

	return []Binding{
		v(ActionInsert, GroupWrite, "write prose here: comment on the line, reply on a thread", "comment", "i"),
		v(ActionPublish, GroupWrite, "publish: a page names what goes out, a second key sends it", "publish", "P"),
		v(ActionBranches, GroupBranch, "the branches and their checks", "branches", "b b"),
		v(ActionBranchDelete, GroupBranch, "delete a branch, after a confirm", "", "b d"),
		v(ActionInfo, GroupGeneral, "about what is open", "info", "g i"),
	}
}

// Disagreements lists every tool binding that reuses a key the shared table gives another action
// on an overlapping screen, and every one that puts a reserved key to another use on any screen
// (Unreserved). A tool's TestKeymap_AgreesWithShared fails on any, which is what keeps one key
// meaning one thing across the family.
func Disagreements(shared, tool []Binding) []string {
	out := Unreserved(tool)

	for _, t := range tool {
		for _, s := range shared {
			if s.Action == t.Action || !s.Contexts.Overlaps(t.Contexts) {
				continue
			}

			for _, tk := range t.Keys {
				for _, sk := range s.Keys {
					if slices.Equal(split(tk), split(sk)) {
						out = append(out, label(tk)+" is "+t.Action+" here and "+s.Action+" in the shared keymap")
					}
				}
			}
		}
	}

	return out
}
