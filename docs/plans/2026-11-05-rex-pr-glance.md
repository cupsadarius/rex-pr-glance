# rex-pr-glance — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A Rex plugin that opens PR Glance beside the focused terminal block and keeps it pointed at the block the user works in.

**Architecture:** Two parts in one repo. `init.lua` (a Lua module loaded from `~/.config/rex/init.lua`) defines the `pr_glance` action that opens, focuses or closes the pane. A Go binary `rex-pr-glance` runs in that pane: it imports `github.com/cupsadarius/pr-glance/glance` and supplies a resolver that polls `rex session inspect --json` to find the working block, its directory, and whether the pane is visible.

**Tech Stack:** Go 1.25, `github.com/cupsadarius/pr-glance v0.2.0`, Rex Lua config API, `rex` CLI, GoReleaser v2.

**Spec:** `herdr-pr-glance/docs/superpowers/specs/2026-11-05-split-and-readiness-overview-design.md`, Part 1 "`rex-pr-glance` (Rex wrapper)".

## Global Constraints

- Documented Rex interfaces only: `init.lua` API (`rex.action`, `rex.call`, `rex.layout.block`) and the `rex` CLI. No block plugin.
- No TUI, GitHub or git logic in this repo: it calls `glance.Run` and `glance.Checkout`.
- Binary env: `REX_SESSION` and `REX_BLOCK` (Rex sets both in every terminal). Missing either → exit with a clear error.
- Cache: `$XDG_STATE_HOME/rex-pr-glance`, default `~/.local/state/rex-pr-glance`.
- Pane label `pr-glance`; default split side right, `ratio = 0.65` (the anchor keeps 65%).
- Release: `v0.1.0`; 4 archives (darwin/linux × amd64/arm64) + `checksums.txt`; `scripts/install.sh` downloads the archive for `VERSION` and checks SHA-256 (copied from herdr-pr-glance).

## Rulings against the spec

- **Polling, not `rex events`.** The core already calls `Resolve` every 2 s; one `rex session inspect --json` per call gives the active window, each window's `focused_block_id` and the attached clients. An event stream adds a goroutine and reconnect logic for no visible gain. Cost if wrong: up to 2 s lag on a focus switch, same as Herdr.
- **Directory from `process`, then `pwd`.** `rex block call process` gives the foreground process cwd (works without OSC 7); `pwd` is the fallback. Same order as `rex-worktrunk`.

## Review Focus

1. Focus moves to the Glance pane itself → keep the previous working block (test `TestSelectKeepsPreviousWhenSelfFocused`).
2. The working block closes → fall back to another terminal block in the same window, else `NoWorkingPane` (test `TestSelectFallsBackWhenWorkingClosed`).
3. Glance window not active, or only CLI clients attached → `Visible = false`, so no GitHub polling (test `TestSelectVisibility`).
4. `pwd` returns a `file://host/path%20with%20space` URL → decoded path (test `TestDecodePWD`).
5. `rex` call fails (server restart) → `Resolve` returns the error; the core shows it and retries; no crash.

---

### Task 1: Resolver (Go)

**Files:** `go.mod`, `internal/rex/resolver.go`, `internal/rex/resolver_test.go`, `internal/rex/testdata/session.json`

**Interfaces:**
- `type Runner interface{ Run(ctx context.Context, cwd string, argv ...string) (string, error) }` (satisfied by `command.ExecRunner`)
- `func Select(v View, self, prev string) (working string, visible bool)` — pure.
- `type Resolver struct{...}`; `func NewResolver(r Runner, session, self, initial string) *Resolver`; `func (r *Resolver) Resolve(ctx) (model.Source, error)`.
- `func decodePWD(url string) string`.

- [ ] Step 1: capture a real `rex session inspect --json` into `testdata/session.json` (two windows; Glance window with 3 terminal blocks). Write tests:
  - `TestSelectFocusedTerminal`: focused block in own window is a terminal ≠ self → working = focused.
  - `TestSelectKeepsPreviousWhenSelfFocused`: focused == self, prev in window → prev.
  - `TestSelectFallsBackWhenWorkingClosed`: focused == self, prev not in window → first other terminal block in layout order; none → "".
  - `TestSelectVisibility`: visible only when `active_window_id` is own window AND a client with `info.kind != "cli"` exists.
  - `TestDecodePWD`: `file://Host.local/Users/a%20b` → `/Users/a b`.
  - `TestResolveUsesProcessCWD`: fake runner returns session JSON and a `process` reply with `foreground.cwd`; Source.CWD equals it, `PaneID` = working block, `Visible` from Select; with a `process` error it falls back to `pwd`.
- [ ] Step 2: `go test ./...` → FAIL (undefined).
- [ ] Step 3: implement. `Resolve`: `rex session inspect SESSION --json` → `Select` → remember working → `rex block call process -b ID` (`foreground.cwd`, else `child.cwd`) → else `rex block call pwd -b ID` → `glance.Checkout(ctx, model.Source{PaneID, CWD, Visible})`; no working block → `EmptyReason = model.NoWorkingPane`.
- [ ] Step 4: `go test ./...` → PASS. Commit.

### Task 2: Binary

**Files:** `cmd/rex-pr-glance/main.go`, `cmd/rex-pr-glance/main_test.go`

- `rex-pr-glance view [--from BLOCK]`, `rex-pr-glance version`. `view` needs `REX_SESSION`, `REX_BLOCK`; zoom runs `rex zoom -b $REX_BLOCK` bounded by `glance.ActionTimeout`.
- [ ] Tests: `TestVersionAndUnknownSubcommands`; `TestViewRequiresRexEnvironment` (unset `REX_SESSION` → error mentions `REX_SESSION`); `TestStateDir` (XDG + HOME fallback → `rex-pr-glance`). RED, implement, GREEN, commit.

### Task 3: init.lua

**Files:** `init.lua`

- `M.setup(opts)` with options `ratio` (0.65), `bin` (default `<module dir>/bin/rex-pr-glance`), `label` (`"pr-glance"`); unknown options warn like rex-worktrunk.
- Action `pr_glance` (title "PR Glance: toggle", category "PR Glance"):
  - find a block labelled `label` in `ctx.window_id` via `rex.call("session.view", {session_id})`;
  - none → `session.new_split{anchor_id = ctx.block_id, direction = "horizontal", side = "after", ratio, focus = false, layout = rex.layout.block{flavor = shell, label = label, options = {cwd = <focused dir>, command = {bin, "view", "--from", ctx.block_id}}}}`;
  - exists and focused → `block.close`; exists, not focused → `session.focus_block`.
- [ ] Verify by hand: symlink repo to `~/.config/rex/rex-pr-glance`, add `require("rex-pr-glance").setup{}` to a scratch config? No: test with `rex do pr_glance` after `rex config reload` only with the owner's go-ahead (it edits their Rex config). Otherwise verify with `luac -p init.lua` if available and a review read. Commit.

### Task 4: Release plumbing and docs

**Files:** `scripts/install.sh`, `internal/installtest/install_test.go`, `.goreleaser.yaml`, `.github/workflows/{ci,release}.yml`, `VERSION` (`0.1.0`), `README.md`, `.gitignore`

- Copy from herdr-pr-glance, rename binary/repo/project to `rex-pr-glance`; release.yml checks tag vs `VERSION` only.
- README: install (`git clone … ~/.config/rex/rex-pr-glance && sh scripts/install.sh`), `init.lua` snippet with `rex.bind("cmd+g", "pr_glance")`, options, remote-server note, controls pointer to pr-glance.
- [ ] `go test -race ./...` (installtest included) → PASS; `go mod tidy` clean. Commit. Push + tag only with the owner's go-ahead.
