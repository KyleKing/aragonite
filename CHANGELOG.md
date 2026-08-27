## v0.1.1 (2026-08-27)

### Fix

- **lint**: drop nolint:gocritic directives gocritic no longer flags

## v0.1.0 (2026-08-27)

### Feat

- **forge,vcs**: read a pull request by number and guard its checkout
- **forge**: move the gh CLI wrapper out of gh-repo-dashboard
- **vcs**: name the checkout's directory from its summary
- **vcs**: move git and jj behind one interface out of gh-repo-dashboard
- **transport**: add a test seam and mutation guard for HTTP clients
- **forge**: move the pull request model out of gh-repo-dashboard
- **cache**: extract the TTL and disk cache from gh-repo-dashboard

### Refactor

- **forge**: leave the pull request model as data only
