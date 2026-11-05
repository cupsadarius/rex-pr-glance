// Package rex finds the terminal block the user works in, through the rex CLI.
package rex

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/cupsadarius/pr-glance/glance"
	"github.com/cupsadarius/pr-glance/model"
)

const terminal = "com.superlogical.terminal.shell"

type Runner interface {
	Run(ctx context.Context, cwd string, argv ...string) (string, error)
}

// View is the part of `rex session inspect --json` the resolver reads.
type View struct {
	ActiveWindowID string `json:"active_window_id"`
	Clients        []struct {
		Info struct {
			Kind string `json:"kind"`
		} `json:"info"`
	} `json:"clients"`
	Windows []struct {
		WindowID       string `json:"window_id"`
		FocusedBlockID string `json:"focused_block_id"`
		ZoomedBlockID  string `json:"zoomed_block_id"`
		Layers         []struct {
			Blocks []struct {
				BlockID string `json:"block_id"`
				Flavor  string `json:"flavor"`
			} `json:"blocks"`
		} `json:"layers"`
	} `json:"windows"`
}

// Select picks the working block: the focused terminal in self's window,
// else prev if it is still a terminal there, else the first other terminal.
// Visible is true when self's window is the session's active window, no other
// block is zoomed over it, and a Rex app is attached. The app attaches to every
// session, so a pane in a session the app does not show still counts as visible.
func Select(v View, self, prev string) (working string, visible bool) {
	for _, w := range v.Windows {
		var terminals []string
		mine := false
		for _, l := range w.Layers {
			for _, b := range l.Blocks {
				if b.BlockID == self {
					mine = true
				} else if b.Flavor == terminal {
					terminals = append(terminals, b.BlockID)
				}
			}
		}
		if !mine {
			continue
		}
		has := func(id string) bool {
			for _, t := range terminals {
				if t == id {
					return true
				}
			}
			return false
		}
		switch {
		case has(w.FocusedBlockID):
			working = w.FocusedBlockID
		case has(prev):
			working = prev
		case len(terminals) > 0:
			working = terminals[0]
		}
		if w.WindowID == v.ActiveWindowID && (w.ZoomedBlockID == "" || w.ZoomedBlockID == self) {
			for _, c := range v.Clients {
				if c.Info.Kind == "app" {
					visible = true
				}
			}
		}
		return working, visible
	}
	return "", false
}

// decodePWD turns the file:// URL a terminal reports into a plain path.
func decodePWD(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "file" {
		return raw
	}
	return u.Path
}

type Resolver struct {
	runner        Runner
	session, self string
	mu            sync.Mutex
	working       string
}

// NewResolver follows the working block of self's window; initial is the block
// that was focused when the pane opened.
func NewResolver(r Runner, session, self, initial string) *Resolver {
	return &Resolver{runner: r, session: session, self: self, working: initial}
}

func (r *Resolver) Resolve(ctx context.Context) (model.Source, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	out, err := r.runner.Run(ctx, "", "rex", "session", "inspect", r.session, "--json")
	if err != nil {
		return model.Source{}, err
	}
	var v View
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		return model.Source{}, fmt.Errorf("decode rex session: %w", err)
	}
	r.mu.Lock()
	working, visible := Select(v, r.self, r.working)
	r.working = working
	r.mu.Unlock()
	s := model.Source{PaneID: working, Visible: visible}
	if working == "" {
		s.EmptyReason = model.NoWorkingPane
		return s, nil
	}
	s.CWD = r.dir(ctx, working)
	return glance.Checkout(ctx, s)
}

// dir is the foreground process's directory, which needs no shell support,
// else the directory the terminal last reported.
func (r *Resolver) dir(ctx context.Context, block string) string {
	if out, err := r.runner.Run(ctx, "", "rex", "block", "call", "process", "-b", block); err == nil {
		var p struct {
			Foreground, Child struct {
				CWD string `json:"cwd"`
			}
		}
		if json.Unmarshal([]byte(out), &p) == nil {
			if p.Foreground.CWD != "" {
				return p.Foreground.CWD
			}
			if p.Child.CWD != "" {
				return p.Child.CWD
			}
		}
	}
	out, err := r.runner.Run(ctx, "", "rex", "block", "call", "pwd", "-b", block)
	if err != nil {
		return ""
	}
	var p struct {
		PWD string `json:"pwd"`
	}
	if json.Unmarshal([]byte(out), &p) != nil {
		return ""
	}
	return strings.TrimSpace(decodePWD(p.PWD))
}
