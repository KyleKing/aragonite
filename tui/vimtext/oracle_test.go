package vimtext_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// nvimDefaults makes vim behave as nvim does out of the box, for the options
// these cases touch, so vim can stand in where nvim is not installed.
const nvimDefaults = `set encoding=utf-8
set nocompatible nostartofline autoindent nojoinspaces backspace=indent,eol,start
set formatoptions=tcq whichwrap=b,s selection=inclusive
silent! set cpoptions+=_
nnoremap Y y$
`

// oracle is the editor the cases are checked against, nvim before vim, and
// the arguments that run it headless on a script.
func oracle(t *testing.T) (string, []string) {
	t.Helper()

	if bin, err := exec.LookPath("nvim"); err == nil {
		return bin, []string{"--headless", "--clean", "-n", "-es"}
	}

	if bin, err := exec.LookPath("vim"); err == nil {
		return bin, []string{"-N", "-u", "NONE", "-i", "NONE", "-n", "-es"}
	}

	t.Skip("neither nvim nor vim is installed to compare against")

	return "", nil
}

var angle = regexp.MustCompile(`<([A-Za-z-]+)>`)

// vimKeys spells a key sequence for feedkeys, which runs it as typed so each
// command is an undo step of its own, as it is for a person (:normal makes the
// whole sequence one).
func vimKeys(seq string) string {
	var b strings.Builder

	for seq != "" {
		if m := angle.FindStringSubmatchIndex(seq); len(m) > 3 && m[0] == 0 {
			b.WriteString(`\<` + seq[m[2]:m[3]] + `>`)
			seq = seq[m[1]:]

			continue
		}

		switch seq[0] {
		case '"', '\\':
			b.WriteByte('\\')
		}

		b.WriteByte(seq[0])
		seq = seq[1:]
	}

	return b.String()
}

// TestOracle runs every case that ends in normal mode through nvim (or vim set
// up as nvim) and compares the text and the cursor with the field's.
func TestOracle(t *testing.T) {
	t.Parallel()

	bin, args := oracle(t)
	dir := t.TempDir()

	for i, c := range keyCases {
		if c.ends != "" || c.diverges != "" {
			continue
		}

		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			text, line, col := marked(t, c.from)
			base := filepath.Join(dir, strconv.Itoa(i))

			if err := os.WriteFile(base+".txt", []byte(text+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}

			script := nvimDefaults +
				"call cursor(" + strconv.Itoa(line+1) + ", " + strconv.Itoa(col+1) + ")\n" +
				`call feedkeys("` + vimKeys(c.keys) + `", 'xt')` + "\n" +
				"call writefile([line('.'), col('.')], '" + base + ".pos')\n" +
				"w! " + base + ".out\nqa!\n"

			if err := os.WriteFile(base+".vim", []byte(script), 0o600); err != nil {
				t.Fatal(err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()

			//nolint:gosec // the editor found above, on files this test wrote
			cmd := exec.CommandContext(ctx, bin, append(args, "-S", base+".vim", base+".txt")...)
			// vim leaves -es with 1 even after a clean w!, so what it wrote is
			// the verdict and its exit status only explains a missing file.
			runErr := cmd.Run()

			out, err := os.ReadFile(base + ".out") //nolint:gosec // written by the oracle
			if err != nil {
				t.Fatalf("%s wrote nothing (%v): %v", bin, runErr, err)
			}

			at, err := os.ReadFile(base + ".pos") //nolint:gosec // written by the oracle
			if err != nil {
				t.Fatal(err)
			}

			p := strings.Fields(string(at))
			l, _ := strconv.Atoi(p[0])  //nolint:errcheck // vim wrote a number
			cl, _ := strconv.Atoi(p[1]) //nolint:errcheck // vim wrote a number
			want := mark(strings.TrimSuffix(string(out), "\n"), l-1, cl-1)

			got, _ := play(t, c)
			if got != want {
				t.Errorf("%q on %q: field %q, %s %q", c.keys, c.from, got, filepath.Base(bin), want)
			}
		})
	}
}
