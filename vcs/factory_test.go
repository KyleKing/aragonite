package vcs_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/kyleking/aragonite/vcs"
)

//nolint:paralleltest // stubs the package-global jj-availability check
func TestDetectVCSType(t *testing.T) {
	tests := []struct {
		setup       func(dir string) error
		name        string
		expected    vcs.Type
		jjAvailable bool
	}{
		{
			name:        "git repo",
			setup:       func(dir string) error { return os.Mkdir(filepath.Join(dir, ".git"), 0o750) },
			jjAvailable: true,
			expected:    vcs.TypeGit,
		},
		{
			name:        "jj repo",
			setup:       func(dir string) error { return os.Mkdir(filepath.Join(dir, ".jj"), 0o750) },
			jjAvailable: true,
			expected:    vcs.TypeJJ,
		},
		{
			name:        "jj-only repo without jj binary stays jj",
			setup:       func(dir string) error { return os.Mkdir(filepath.Join(dir, ".jj"), 0o750) },
			jjAvailable: false,
			expected:    vcs.TypeJJ,
		},
		{
			name: "colocated prefers jj",
			setup: func(dir string) error {
				if err := os.Mkdir(filepath.Join(dir, ".git"), 0o750); err != nil {
					return fmt.Errorf("mkdir .git: %w", err)
				}

				return os.Mkdir(filepath.Join(dir, ".jj"), 0o750)
			},
			jjAvailable: true,
			expected:    vcs.TypeJJ,
		},
		{
			name: "colocated without jj binary falls back to git",
			setup: func(dir string) error {
				if err := os.Mkdir(filepath.Join(dir, ".git"), 0o750); err != nil {
					return fmt.Errorf("mkdir .git: %w", err)
				}

				return os.Mkdir(filepath.Join(dir, ".jj"), 0o750)
			},
			jjAvailable: false,
			expected:    vcs.TypeGit,
		},
		{
			name:        "empty dir defaults to git",
			setup:       func(_ string) error { return nil },
			jjAvailable: true,
			expected:    vcs.TypeGit,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := tt.setup(dir); err != nil {
				t.Fatal(err)
			}
			restore := vcs.SetJJAvailableForTest(tt.jjAvailable)
			defer restore()

			result := vcs.DetectVCSType(dir)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGetOperations(t *testing.T) {
	t.Parallel()
	tests := []struct {
		setup       func(dir string) error
		name        string
		expectedVCS vcs.Type
	}{
		{
			name:        "returns git ops for git repo",
			setup:       func(dir string) error { return os.Mkdir(filepath.Join(dir, ".git"), 0o750) },
			expectedVCS: vcs.TypeGit,
		},
		{
			name:        "returns jj ops for jj repo",
			setup:       func(dir string) error { return os.Mkdir(filepath.Join(dir, ".jj"), 0o750) },
			expectedVCS: vcs.TypeJJ,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			if err := tt.setup(dir); err != nil {
				t.Fatal(err)
			}

			ops := vcs.GetOperations(dir)
			if ops.VCSType() != tt.expectedVCS {
				t.Errorf("expected %v, got %v", tt.expectedVCS, ops.VCSType())
			}
		})
	}
}

func TestIsRepo(t *testing.T) {
	t.Parallel()
	tests := []struct {
		setup    func(dir string) error
		name     string
		expected bool
	}{
		{
			name:     "git repo",
			setup:    func(dir string) error { return os.Mkdir(filepath.Join(dir, ".git"), 0o750) },
			expected: true,
		},
		{
			name:     "jj repo",
			setup:    func(dir string) error { return os.Mkdir(filepath.Join(dir, ".jj"), 0o750) },
			expected: true,
		},
		{
			name:     "not a repo",
			setup:    func(_ string) error { return nil },
			expected: false,
		},
		{
			name:     "has other dot dirs but not vcs",
			setup:    func(dir string) error { return os.Mkdir(filepath.Join(dir, ".config"), 0o750) },
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			if err := tt.setup(dir); err != nil {
				t.Fatal(err)
			}

			result := vcs.IsRepo(dir)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGetGitHubEnv(t *testing.T) {
	t.Parallel()
	tests := []struct {
		setup       func(dir string) error
		name        string
		expectEmpty bool
	}{
		{
			name:        "git repo returns nil",
			setup:       func(dir string) error { return os.Mkdir(filepath.Join(dir, ".git"), 0o750) },
			expectEmpty: true,
		},
		{
			name: "colocated jj repo returns nil",
			setup: func(dir string) error {
				if err := os.Mkdir(filepath.Join(dir, ".git"), 0o750); err != nil {
					return fmt.Errorf("mkdir .git: %w", err)
				}

				return os.Mkdir(filepath.Join(dir, ".jj"), 0o750)
			},
			expectEmpty: true,
		},
		{
			name: "non-colocated jj repo sets GIT_DIR",
			setup: func(dir string) error {
				return os.MkdirAll(filepath.Join(dir, ".jj", "repo", "store", "git"), 0o750)
			},
			expectEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			if err := tt.setup(dir); err != nil {
				t.Fatal(err)
			}

			env := vcs.GetGitHubEnv(dir)
			if tt.expectEmpty && len(env) > 0 {
				t.Errorf("expected empty env, got %v", env)
			}
			if !tt.expectEmpty {
				if len(env) == 0 {
					t.Error("expected GIT_DIR env var")
				} else if env[0] != "GIT_DIR="+filepath.Join(dir, ".jj", "repo", "store", "git") {
					t.Errorf("unexpected env: %v", env)
				}
			}
		})
	}
}
