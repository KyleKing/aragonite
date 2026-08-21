package cache_test

// Stand-ins for the caller-side payloads the cache is generic over. The cache
// only requires that a value round-trips through encoding/json.
type (
	checksStatus struct{ Total, Passing, Failing, Pending int }
	prInfo       struct {
		Title   string
		State   string
		HeadRef string
		Checks  checksStatus
		Number  int
	}
)
