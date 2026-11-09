package hub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nwverse/persona/server/internal/bus"
	"github.com/nwverse/persona/server/internal/db"
	"github.com/nwverse/persona/server/internal/rpc"
)

// fullSendBuffer is a manager whose connection takes no more writes (a sleeping laptop with its TCP send buffer full):
// a write returns only once hangUp is closed.
type fullSendBuffer struct{ hangUp chan struct{} }

func (w fullSendBuffer) Write([]byte) (int, error) {
	<-w.hangUp
	return 0, errors.New("connection closed")
}

// rosterLog is a manager that takes every write, each after delay, and keeps the managers events it got.
type rosterLog struct {
	delay time.Duration
	mu    sync.Mutex
	lists [][]busManagerBrief
}

func (l *rosterLog) Write(b []byte) (int, error) {
	time.Sleep(l.delay)
	var m struct {
		Event  string
		Params struct{ Managers []busManagerBrief }
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return 0, err
	}
	if m.Event == "managers" {
		l.mu.Lock()
		l.lists = append(l.lists, m.Params.Managers)
		l.mu.Unlock()
	}
	return len(b), nil
}

func (l *rosterLog) got() [][]busManagerBrief {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([][]busManagerBrief(nil), l.lists...)
}

// rosterConn makes w the current connection of manager id.
func rosterConn(h *Hub, id string, w interface{ Write([]byte) (int, error) }) {
	h.bus.mu.Lock()
	defer h.bus.mu.Unlock()
	if h.bus.current == nil {
		h.bus.current = map[string]*busConn{}
	}
	h.bus.current[id] = &busConn{ident: db.BusIdentity{ID: id, Role: bus.RoleManager}, peer: rpc.New("h", w)}
}

// A manager whose connection stopped taking writes holds up neither the roster for the others nor the caller: the
// handshake of a manager that just connected (busAccept) and the teardown of one that left (busEnded) run it.
func TestRosterReachesTheOthersPastAStalledManager(t *testing.T) {
	h := newTestHub(t)
	h.bus.idents = &memIdents{}
	stalled := fullSendBuffer{hangUp: make(chan struct{})}
	t.Cleanup(func() { close(stalled.hangUp) }) // the stalled connections hang up: their writes return
	for i := range 8 {
		rosterConn(h, fmt.Sprintf("m-stalled%d", i), stalled)
	}
	healthy := &rosterLog{}
	rosterConn(h, "m-healthy", healthy)

	done := make(chan struct{})
	go func() {
		h.busRosterChanged(context.Background(), nil)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("busRosterChanged waited on a manager whose connection takes no writes")
	}
	eventually(t, 2*time.Second, "the healthy manager got the roster", func() bool { return len(healthy.got()) > 0 })
	time.Sleep(50 * time.Millisecond) // nothing more on the way
	if n := len(healthy.got()); n != 1 {
		t.Fatalf("the healthy manager got %d managers events, want 1", n)
	}
}

// Roster changes in quick succession reach a slow manager in order: it ends with the newest list and never gets an
// older one after a newer one. Each change here adds an identity, so a newer list is a longer one.
func TestRosterChangesReachASlowManagerInOrder(t *testing.T) {
	h := newTestHub(t)
	idents := &memIdents{}
	h.bus.idents = idents
	slow := &rosterLog{delay: 200 * time.Millisecond}
	rosterConn(h, "m-slow", slow)
	ctx := context.Background()
	const changes = 6
	start := time.Now()
	for i := range changes {
		if err := idents.SaveBusIdentity(ctx, db.BusIdentity{ID: fmt.Sprintf("m-%d", i), Role: bus.RoleManager}, fmt.Sprint(i)); err != nil {
			t.Fatal(err)
		}
		h.busRosterChanged(ctx, nil)
	}
	if d := time.Since(start); d > time.Second {
		t.Fatalf("%d roster changes took %s: they waited on the slow manager", changes, d)
	}
	eventually(t, 3*time.Second, "the slow manager got the newest roster", func() bool {
		got := slow.got()
		return len(got) > 0 && len(got[len(got)-1]) == changes
	})
	time.Sleep(2 * slow.delay) // nothing older on the way behind it
	got := slow.got()
	var sizes []string
	for i, list := range got {
		sizes = append(sizes, fmt.Sprint(len(list)))
		if i > 0 && len(list) <= len(got[i-1]) {
			t.Fatalf("the slow manager got an older roster after a newer one: sizes %s", strings.Join(sizes, ","))
		}
	}
	if last := got[len(got)-1]; len(last) != changes {
		t.Fatalf("the slow manager ended with %d managers, want %d (sizes %s)", len(last), changes, strings.Join(sizes, ","))
	}
}
