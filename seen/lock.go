package seen

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"time"
)

// ErrBusy is returned when another process held a set's lock for longer than a save waits.
var ErrBusy = errors.New("read marks are being saved by another process")

const (
	// How long a save waits for another process's save to finish. A save takes milliseconds, so
	// this is long only for a lock that is stuck.
	lockWait = 2 * time.Second
	// The age past which a lock is taken to be left by a process that died holding it, and is
	// broken.
	lockStale = 10 * time.Second
	lockPoll  = 5 * time.Millisecond
)

// lock takes path's lock file, a sibling created exclusively, so it works the same on every
// platform and on a network home directory where flock does not. It returns the release.
func lock(path string) (func() error, error) {
	name := path + ".lock"
	deadline := time.Now().Add(lockWait)

	for {
		took, err := tryLock(name)
		if err != nil {
			return nil, fmt.Errorf("locking %s: %w", path, err)
		}

		if took {
			return func() error {
				if err := os.Remove(name); err != nil {
					return fmt.Errorf("unlocking %s: %w", path, err)
				}

				return nil
			}, nil
		}

		broke, err := breakStale(name)
		if err != nil {
			return nil, fmt.Errorf("breaking a stale lock on %s: %w", path, err)
		}

		if broke {
			continue
		}

		if time.Now().After(deadline) {
			return nil, fmt.Errorf("saving %s: %w", path, ErrBusy)
		}

		time.Sleep(lockPoll)
	}
}

// tryLock creates the lock file and reports whether it did; one that already exists is not an
// error.
func tryLock(name string) (bool, error) {
	//nolint:gosec // beside the caller's own state file
	f, err := os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, filePerm)
	if errors.Is(err, fs.ErrExist) {
		return false, nil
	}

	if err != nil {
		return false, err //nolint:wrapcheck // lock wraps it with the path
	}

	_, werr := f.WriteString(strconv.Itoa(os.Getpid()))
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}

	if werr != nil {
		return false, errors.Join(werr, os.Remove(name))
	}

	return true, nil
}

// breakStale removes a lock older than lockStale and reports whether it did.
func breakStale(name string) (bool, error) {
	info, err := os.Stat(name)
	if err != nil || time.Since(info.ModTime()) <= lockStale {
		return false, nil //nolint:nilerr // a lock gone since the create is retried, not an error
	}

	if err := os.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, err //nolint:wrapcheck // lock wraps it with the path
	}

	return true, nil
}
