package main

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// execute runs the CLI with args against a fresh command tree, which also
// resets the package-level flag variables to their defaults.
func execute(t *testing.T, args ...string) error {
	t.Helper()
	cmd := newRootCmd()
	cmd.SetArgs(args)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	return cmd.Execute()
}

func writePhoto(t *testing.T, path string, when time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("fake-jpeg "+path), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatal(err)
	}
}

func TestRunFlagValidation(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file.jpg")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "src")
	if err := os.Mkdir(src, 0o755); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "dest")

	cases := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"no flags", []string{"copy"}, "--src and --dest are required"},
		{"src only", []string{"copy", "--src", src}, "--src and --dest are required"},
		{"dest only", []string{"move", "--dest", dest}, "--src and --dest are required"},
		{"src missing", []string{"copy", "-s", filepath.Join(dir, "nope"), "-d", dest}, "is not a directory"},
		{"src is a file", []string{"copy", "-s", file, "-d", dest}, "is not a directory"},
		{"dest inside src", []string{"copy", "-s", src, "-d", filepath.Join(src, "out")}, "is inside source"},
		{"unknown command", []string{"frobnicate"}, "unknown command"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := execute(t, tc.args...)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, tc.wantErr)
			}
		})
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Errorf("a rejected run created the destination: %v", err)
	}
}

func TestRunCopySucceeds(t *testing.T) {
	src, dest := t.TempDir(), t.TempDir()
	when := time.Date(2023, 3, 15, 12, 0, 0, 0, time.Local)
	writePhoto(t, filepath.Join(src, "a.jpg"), when)

	if err := execute(t, "copy", "--src", src, "--dest", dest, "--workers", "2"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "2023", "2023_03", "2023_03_15", "a.jpg")); err != nil {
		t.Fatal(err)
	}
}

func TestRunDryRunLeavesSourceAndDestAlone(t *testing.T) {
	src, dest := t.TempDir(), t.TempDir()
	photo := filepath.Join(src, "a.jpg")
	writePhoto(t, photo, time.Date(2023, 3, 15, 12, 0, 0, 0, time.Local))

	if err := execute(t, "move", "-s", src, "-d", dest, "-n"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(photo); err != nil {
		t.Fatalf("dry-run move touched the source: %v", err)
	}
	if entries, _ := os.ReadDir(dest); len(entries) != 0 {
		t.Fatalf("dry-run wrote to dest: %v", entries)
	}
}

func TestRunFailedFilesReturnError(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs POSIX permissions and a non-root user")
	}
	src, dest := t.TempDir(), t.TempDir()
	writePhoto(t, filepath.Join(src, "a.jpg"), time.Date(2023, 3, 15, 12, 0, 0, 0, time.Local))
	// The year folder exists but can't be written into, so the file fails.
	year := filepath.Join(dest, "2023")
	if err := os.Mkdir(year, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(year, 0o755) })

	err := execute(t, "copy", "-s", src, "-d", dest)
	if err == nil || !strings.Contains(err.Error(), "1 files failed") {
		t.Fatalf("err = %v, want \"1 files failed\"", err)
	}
}

func TestRootRegistersCommandsAndFlags(t *testing.T) {
	root := newRootCmd()
	for _, name := range []string{"copy", "move", "version"} {
		if cmd, _, err := root.Find([]string{name}); err != nil || cmd.Name() != name {
			t.Errorf("subcommand %q not registered: %v", name, err)
		}
	}
	for flag, short := range map[string]string{
		"src": "s", "dest": "d", "dry-run": "n", "workers": "w", "verbose": "v",
	} {
		f := root.PersistentFlags().Lookup(flag)
		if f == nil || f.Shorthand != short {
			t.Errorf("--%s missing or shorthand != -%s", flag, short)
		}
	}
	if got := root.PersistentFlags().Lookup("workers").DefValue; got != "4" {
		t.Errorf("--workers default = %s, want 4", got)
	}
}
