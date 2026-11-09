package node

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nwverse/persona/server/internal/host"
	"github.com/nwverse/persona/server/internal/tmux"
)

func TestTmuxPaneOf(t *testing.T) {
	for _, c := range []struct{ target, want string }{
		{"orc:@11.%11", "tmux:%11"},
		{"%3", "tmux:%3"},
		{"", ""},
		{"orc:@0", ""}, // no pane: window only
	} {
		if got := tmuxPaneOf(c.target); got != c.want {
			t.Errorf("tmuxPaneOf(%q) = %q, want %q", c.target, got, c.want)
		}
	}
}

// A fake tmux prints a dialog for a capture and records the keys it is asked to press.
func TestTmuxHostReadsAndPresses(t *testing.T) {
	dir := t.TempDir()
	calls := filepath.Join(dir, "calls")
	bin := filepath.Join(dir, "tmux")
	script := "#!/bin/sh\ncase \"$1\" in\n-V) echo 'tmux 3.6a' ;;\ncapture-pane) echo 'Do you want to proceed?' ;;\nsend-keys) echo \"$@\" >> " + calls + " ;;\nesac\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	h := &tmuxHost{c: tmux.Client{Bin: bin}}
	pane := host.PaneRef{Kind: host.Tmux, ID: "%3"}

	text, err := h.Read(context.Background(), pane, host.ReadOpts{Lines: 30})
	if err != nil || !strings.Contains(text, "Do you want to proceed?") {
		t.Fatalf("Read = %q, %v", text, err)
	}
	if err := h.SendKeys(context.Background(), pane, "enter"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(calls)
	if err != nil || !strings.Contains(string(b), "send-keys -t %3 Enter") {
		t.Fatalf("keys pressed = %q, %v", b, err)
	}
}
