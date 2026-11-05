package rex

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/cupsadarius/pr-glance/model"
)

func view(t *testing.T, edit func(*View)) View {
	t.Helper()
	b, err := os.ReadFile("testdata/session.json")
	if err != nil {
		t.Fatal(err)
	}
	var v View
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	if edit != nil {
		edit(&v)
	}
	return v
}

func TestSelectFocusedTerminal(t *testing.T) {
	if got, _ := Select(view(t, nil), "block:glance", "block:claude"); got != "block:editor" {
		t.Fatalf("working = %q, want block:editor", got)
	}
}

func TestSelectKeepsPreviousWhenSelfFocused(t *testing.T) {
	v := view(t, func(v *View) { v.Windows[0].FocusedBlockID = "block:glance" })
	if got, _ := Select(v, "block:glance", "block:claude"); got != "block:claude" {
		t.Fatalf("working = %q, want block:claude", got)
	}
}

func TestSelectFallsBackWhenWorkingClosed(t *testing.T) {
	v := view(t, func(v *View) { v.Windows[0].FocusedBlockID = "block:glance" })
	if got, _ := Select(v, "block:glance", "block:gone"); got != "block:editor" {
		t.Fatalf("working = %q, want block:editor", got)
	}
	// A previous block in another window does not count as this window's.
	if got, _ := Select(v, "block:glance", "block:other"); got != "block:editor" {
		t.Fatalf("working = %q, want block:editor", got)
	}
	alone := view(t, func(v *View) {
		v.Windows[0].FocusedBlockID = "block:glance"
		v.Windows[0].Layers[0].Blocks = v.Windows[0].Layers[0].Blocks[2:]
	})
	if got, _ := Select(alone, "block:glance", ""); got != "" {
		t.Fatalf("working = %q, want none", got)
	}
}

func TestSelectIgnoresNonTerminalFocus(t *testing.T) {
	v := view(t, nil)
	if got, _ := Select(v, "block:other", ""); got != "" {
		t.Fatalf("working = %q, want none: the only other block is not a terminal", got)
	}
}

func TestSelectVisibility(t *testing.T) {
	if _, vis := Select(view(t, nil), "block:glance", ""); !vis {
		t.Fatal("active window with an app client is not visible")
	}
	if _, vis := Select(view(t, func(v *View) { v.ActiveWindowID = "window:w2" }), "block:glance", ""); vis {
		t.Fatal("inactive window is visible")
	}
	cliOnly := view(t, func(v *View) { v.Clients = v.Clients[1:] })
	if _, vis := Select(cliOnly, "block:glance", ""); vis {
		t.Fatal("a session with only CLI clients is visible")
	}
	agentOnly := view(t, func(v *View) { v.Clients[0].Info.Kind = "agent"; v.Clients = v.Clients[:1] })
	if _, vis := Select(agentOnly, "block:glance", ""); vis {
		t.Fatal("a session with only an agent client is visible")
	}
	zoomedOther := view(t, func(v *View) { v.Windows[0].ZoomedBlockID = "block:editor" })
	if _, vis := Select(zoomedOther, "block:glance", ""); vis {
		t.Fatal("the pane is visible while another block is zoomed over it")
	}
	zoomedSelf := view(t, func(v *View) { v.Windows[0].ZoomedBlockID = "block:glance" })
	if _, vis := Select(zoomedSelf, "block:glance", ""); !vis {
		t.Fatal("the zoomed pane is not visible")
	}
}

func TestResolveFallsBackToChildCWD(t *testing.T) {
	b, _ := os.ReadFile("testdata/session.json")
	dir := t.TempDir()
	f := &fakeRunner{session: string(b), process: `{"foreground":{},"child":{"cwd":"` + dir + `"}}`}
	s, err := NewResolver(f, "session:s1", "block:glance", "").Resolve(context.Background())
	if err != nil || s.CWD != dir {
		t.Fatalf("source = %+v, err = %v", s, err)
	}
}

func TestDecodePWD(t *testing.T) {
	if got := decodePWD("file://Host.local/Users/a%20b/c"); got != "/Users/a b/c" {
		t.Fatalf("got %q", got)
	}
	if got := decodePWD("/plain/path"); got != "/plain/path" {
		t.Fatalf("got %q", got)
	}
}

type fakeRunner struct {
	session    string
	process    string
	processErr error
	pwd        string
	calls      []string
}

func (f *fakeRunner) Run(_ context.Context, _ string, argv ...string) (string, error) {
	line := strings.Join(argv, " ")
	f.calls = append(f.calls, line)
	switch {
	case strings.HasPrefix(line, "rex session inspect session:s1 --json"):
		return f.session, nil
	case strings.HasPrefix(line, "rex block call process -b block:editor"):
		return f.process, f.processErr
	case strings.HasPrefix(line, "rex block call pwd -b block:editor"):
		return f.pwd, nil
	}
	return "", errors.New("unexpected: " + line)
}

func TestResolveUsesProcessCWD(t *testing.T) {
	b, _ := os.ReadFile("testdata/session.json")
	dir := t.TempDir()
	f := &fakeRunner{session: string(b), process: `{"foreground":{"cwd":"` + dir + `"},"child":{"cwd":"/elsewhere"}}`}
	s, err := NewResolver(f, "session:s1", "block:glance", "").Resolve(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if s.CWD != dir || s.PaneID != "block:editor" || !s.Visible || s.EmptyReason != model.NotGit {
		t.Fatalf("source = %+v", s)
	}
	f.processErr = errors.New("no process")
	f.pwd = `{"pwd":"file://Host.local` + dir + `"}`
	if s, err = NewResolver(f, "session:s1", "block:glance", "").Resolve(context.Background()); err != nil || s.CWD != dir {
		t.Fatalf("pwd fallback: source = %+v, err = %v", s, err)
	}
}

func TestResolveWithoutWorkingBlock(t *testing.T) {
	f := &fakeRunner{session: `{"active_window_id":"window:w1","windows":[{"window_id":"window:w1","focused_block_id":"block:glance","layers":[{"blocks":[{"block_id":"block:glance","flavor":"com.superlogical.terminal.shell"}]}]}]}`}
	s, err := NewResolver(f, "session:s1", "block:glance", "").Resolve(context.Background())
	if err != nil || s.EmptyReason != model.NoWorkingPane {
		t.Fatalf("source = %+v, err = %v", s, err)
	}
}

func TestResolveReportsRexFailure(t *testing.T) {
	f := &fakeRunner{}
	if _, err := NewResolver(f, "session:bad", "block:glance", "").Resolve(context.Background()); err == nil {
		t.Fatal("a failed rex call was not reported")
	}
}
