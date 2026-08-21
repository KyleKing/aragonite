package github

import (
	"time"

	"github.com/kyleking/aragonite/cache"
	"github.com/kyleking/aragonite/forge"
)

const (
	defaultTTL  = 5 * time.Minute
	workflowTTL = 2 * time.Minute
)

// What a gh call said, keyed by who else may read it: RemoteScope for anything
// read off the remote, so parallel checkouts of one repo share a fetch.
//
//nolint:gochecknoglobals // one process-wide cache each, which is the point of registering them
var (
	DefaultBranchCICache = cache.NewRegistered[*forge.DefaultBranchCI](workflowTTL)
	MergedPRHeadsCache   = cache.NewRegistered[map[string]string](defaultTTL)
	PRCache              = cache.NewRegistered[*forge.PullRequest](defaultTTL)
	PRDetailCache        = cache.NewRegistered[*forge.PRDetail](defaultTTL)
	PRListCache          = cache.NewRegistered[[]forge.PullRequest](defaultTTL)
	PRPreviewCache       = cache.NewRegistered[*forge.PRPreview](defaultTTL)
	PRSearchCache        = cache.NewRegistered[[]forge.PullRequest](defaultTTL)
	WorkflowCache        = cache.NewRegistered[*forge.WorkflowSummary](workflowTTL)
)
