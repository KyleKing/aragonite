package cache_test

// Stand-ins for the caller-side payloads the cache is generic over. The cache
// only requires that a value round-trips through encoding/json.
type (
	checksStatus struct{ Total, Passing, Failing, Pending int }
	prInfo       struct {
		Number  int
		Title   string
		State   string
		HeadRef string
		Checks  checksStatus
	}
	branchInfo struct{ Name string }
	commitInfo struct{ Subject string }
)
