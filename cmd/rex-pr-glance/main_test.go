package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionAndUnknownSubcommands(t *testing.T) {
	var out bytes.Buffer
	original := version
	version = "1.2.3-test"
	t.Cleanup(func() { version = original })
	if err := run([]string{"version"}, &out); err != nil || out.String() != "1.2.3-test\n" {
		t.Fatalf("version output = %q, err = %v", out.String(), err)
	}
	for _, args := range [][]string{{}, {"nope"}, {"view", "--nope"}, {"view", "extra"}} {
		out.Reset()
		if err := run(args, &out); err == nil {
			t.Fatalf("%v accepted", args)
		}
		if out.Len() != 0 {
			t.Fatalf("%v wrote to stdout: %q", args, out.String())
		}
	}
}

func TestViewRequiresRexEnvironment(t *testing.T) {
	t.Setenv("REX_SESSION", "")
	t.Setenv("REX_BLOCK", "block:b1")
	err := run([]string{"view"}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "REX_SESSION") {
		t.Fatalf("err = %v", err)
	}
	t.Setenv("REX_SESSION", "session:s1")
	t.Setenv("REX_BLOCK", "")
	err = run([]string{"view"}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "REX_BLOCK") {
		t.Fatalf("err = %v", err)
	}
}

func TestStateDir(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_STATE_HOME", base)
	dir, err := stateDir()
	if err != nil || dir != filepath.Join(base, "rex-pr-glance") {
		t.Fatalf("dir = %q, err = %v", dir, err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", home)
	if dir, err = stateDir(); err != nil || dir != filepath.Join(home, ".local", "state", "rex-pr-glance") {
		t.Fatalf("dir = %q, err = %v", dir, err)
	}
}
