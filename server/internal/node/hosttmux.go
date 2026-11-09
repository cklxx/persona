package node

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nwverse/persona/server/internal/host"
	"github.com/nwverse/persona/server/internal/tmux"
)

// tmuxHost is a pane of a tmux server Persona did not open: a Claude Code the user started in tmux (#2562). Only what
// an approval needs — read the pane's screen and press keys in it. The pane is named by tmux itself ("%11"), as
// Claude Code wrote it down in sessions/<pid>.json ("tmux": "orc:@11.%11"); the pane's id is unique within its server,
// which is the default one unless a process's TMUX says otherwise. The layout is not read and no space, slot or agent
// comes from here: the session that runs there is listed by claudebg.go, on its own pid file.
type tmuxHost struct {
	c tmux.Client
	// socket is the server's socket when the default one is not used (TMUX in the process's environment), "" for the
	// default server. Panes from different servers are told apart by the pane's own id only, which repeats across them.
	socket string
}

var (
	_ host.Host        = (*tmuxHost)(nil)
	_ host.ScreenDriver = (*tmuxHost)(nil)
)

func newTmuxHost() *tmuxHost { return &tmuxHost{} }

func (h *tmuxHost) Kind() string    { return host.Tmux }
func (h *tmuxHost) PaneEnv() string { return "TMUX_PANE" }

// CanDriveScreen: a tmux pane is read and typed into through the tmux command, which works off the pane's own server.
func (h *tmuxHost) CanDriveScreen() bool { return true }

func (h *tmuxHost) Version(ctx context.Context) string { return h.client().Version(ctx) }

func (h *tmuxHost) client() tmux.Client { return tmux.Client{Bin: h.c.Bin, Socket: h.socket} }

// Snapshot lists nothing: tmux sessions the user opened are not Persona's spaces or slots, and the agents in them are
// listed by claudebg.go. It never fails: a machine without tmux simply has no panes this host can reach.
func (h *tmuxHost) Snapshot(context.Context) (*host.Layout, error) { return &host.Layout{}, nil }

// Agents lists nothing, for Snapshot's reason.
func (h *tmuxHost) Agents(context.Context) ([]host.Agent, error) { return nil, nil }

// Agent is none: this host detects no agents.
func (h *tmuxHost) Agent(context.Context, host.PaneRef) (*host.Agent, error) {
	return nil, fmt.Errorf("%w: tmux", host.ErrNoAgent)
}

func (h *tmuxHost) Read(ctx context.Context, pane host.PaneRef, o host.ReadOpts) (string, error) {
	text, err := h.client().Capture(ctx, pane.ID, o.Lines, o.ANSI)
	return text, tmuxErr(err)
}

func (h *tmuxHost) SendText(ctx context.Context, pane host.PaneRef, text string) error {
	return tmuxErr(h.client().SendText(ctx, pane.ID, text))
}

func (h *tmuxHost) SendKeys(ctx context.Context, pane host.PaneRef, keys ...string) error {
	return tmuxErr(h.client().Keys(ctx, pane.ID, keys...))
}

func (h *tmuxHost) Prompt(ctx context.Context, pane host.PaneRef, text string) error {
	return tmuxErr(h.client().Prompt(ctx, pane.ID, text))
}

// PaneProcesses is not read: a tmux pane's processes are not needed to read its screen or press its keys.
func (h *tmuxHost) PaneProcesses(context.Context, host.PaneRef) ([]int, error) { return nil, nil }

// tmuxErr marks a tmux that is gone as the host being gone: the pane with it.
func tmuxErr(err error) error {
	if errors.Is(err, tmux.ErrNoServer) {
		return host.Mark(err, host.ErrNotRunning)
	}
	return err
}

// tmuxPaneOf is the pane tmux names in Claude Code's own record ("orc:@11.%11", its session:window.pane) as this
// node's tmux pane ref ("tmux:%11"), "" when it names none. The pane id is unique within its server, so the session and
// window before it add nothing.
func tmuxPaneOf(target string) string {
	id := target
	if i := strings.LastIndex(id, "."); i >= 0 {
		id = id[i+1:]
	}
	if !strings.HasPrefix(id, "%") || len(id) < 2 {
		return ""
	}
	return host.PaneRef{Kind: host.Tmux, ID: id}.String()
}
