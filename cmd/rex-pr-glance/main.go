// Command rex-pr-glance shows the pull request of the Rex terminal block the
// user works in.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/cupsadarius/pr-glance/command"
	"github.com/cupsadarius/pr-glance/glance"
	"github.com/cupsadarius/rex-pr-glance/internal/rex"
)

// version is replaced at release build time with -ldflags "-X main.version=...".
var version = "dev"

const usage = "usage: rex-pr-glance view [--from BLOCK] | version"

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "rex-pr-glance: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New(usage)
	}
	switch args[0] {
	case "view":
		return view(context.Background(), args[1:])
	case "version":
		_, err := fmt.Fprintln(stdout, version)
		return err
	}
	return fmt.Errorf("unknown subcommand %q\n%s", args[0], usage)
}

func view(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("view", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	from := flags.String("from", "", "block that was focused when the pane opened")
	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("%w\n%s", err, usage)
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("unexpected argument %q\n%s", flags.Arg(0), usage)
	}
	session, self := os.Getenv("REX_SESSION"), os.Getenv("REX_BLOCK")
	if session == "" || self == "" {
		return errors.New("REX_SESSION and REX_BLOCK are not set; run inside a Rex terminal")
	}
	dir, err := stateDir()
	if err != nil {
		return err
	}
	runner := command.ExecRunner{}
	zoom := func() error {
		ctx, cancel := context.WithTimeout(ctx, glance.ActionTimeout)
		defer cancel()
		_, err := runner.Run(ctx, "", "rex", "zoom", "-b", self)
		return err
	}
	resolver := rex.NewResolver(runner, session, self, *from)
	return glance.Run(ctx, glance.Options{Resolver: resolver, StateDir: dir, Zoom: zoom})
}

// stateDir is the cache home: $XDG_STATE_HOME/rex-pr-glance, else
// ~/.local/state/rex-pr-glance.
func stateDir() (string, error) {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "state")
	}
	dir := filepath.Join(base, "rex-pr-glance")
	return dir, os.MkdirAll(dir, 0o755)
}
