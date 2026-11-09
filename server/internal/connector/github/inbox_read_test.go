package github

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"time"
)

// When a thing counts as read on GitHub (#2149): only when its own newest notification is gone from the unread list,
// never while that notification is still unread somewhere GitHub's paging hid it.

// stamp is t as GitHub writes it.
func stamp(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// skipped is n unread notifications the inbox leaves out (subscribed), numbered from first, all updated at at.
func skipped(first, n int, at time.Time) []string {
	out := make([]string, n)
	for i := range out {
		id := strconv.Itoa(first + i)
		out[i] = note(id, "subscribed", "Issue", "https://api.github.com/repos/example-org/example-repo/issues/"+id, stamp(at))
	}
	return out
}

// ciNote is a CheckSuite notification (no URL: the branch is in the title).
func ciNote(id, title string, at time.Time) string {
	b, _ := json.Marshal(map[string]any{"id": id, "reason": "ci_activity", "updated_at": stamp(at),
		"subject":    map[string]any{"title": title, "url": nil, "type": "CheckSuite"},
		"repository": map[string]string{"full_name": "example-org/example-repo", "html_url": "https://github.com/example-org/example-repo"}})
	return string(b)
}

func cat(lists ...[]string) []string {
	var out []string
	for _, l := range lists {
		out = append(out, l...)
	}
	return out
}

// pollOnce polls and fails the test on an error.
func pollOnce(t *testing.T, c *Connector) {
	t.Helper()
	if _, _, err := c.poll(context.Background()); err != nil {
		t.Fatalf("poll: %v", err)
	}
}

// A CI item the follow-up moved to a later failed run (someone else's push: GitHub sent no notification of it) is
// not read because its own notification fell past the pages read, nor because that notification was read: the later
// run is news. A read item the follow-up moves is unread again.
func TestCIItemPastItsNotification(t *testing.T) {
	oldRecheck := ciRecheck
	t.Cleanup(func() { ciRecheck = oldRecheck })
	now := time.Now().UTC().Truncate(time.Second)
	notified, between, later := now.Add(-3*time.Hour), now.Add(-2*time.Hour), now.Add(-time.Hour)
	const thread = "ci:example-org/example-repo@main"
	ci := ciNote("31", "CI workflow run failed for main branch", notified)
	laterRun := `[{"name":"CI","status":"completed","conclusion":"failure","updated_at":"` + stamp(later) + `"}]`
	item := func(t *testing.T, c *Connector) (at time.Time, read bool) {
		t.Helper()
		th, ok := things(t, c)[thread]
		if !ok || th.Done != "" {
			t.Fatalf("the CI item: in the inbox %v, %+v", ok, th)
		}
		return th.At, th.Read
	}
	// moveOn has the follow-up check the item now and fold in the later run.
	moveOn := func(t *testing.T, f *pagedNotifications, c *Connector) {
		t.Helper()
		f.mu.Lock()
		f.runs = laterRun
		f.mu.Unlock()
		ciRecheck = 0
		moved := c.followUpCI(context.Background())
		ciRecheck = oldRecheck
		if at, _ := item(t, c); !moved || !at.Equal(later) {
			t.Fatalf("the follow-up did not fold in the later run: moved %v, at %v", moved, at)
		}
	}
	made := func(t *testing.T) (*pagedNotifications, *Connector) {
		f := &pagedNotifications{list: []string{ci}}
		c := f.server(t)
		pollOnce(t, c)
		if at, read := item(t, c); !at.Equal(notified) || read {
			t.Fatalf("first poll: at %v, read %v", at, read)
		}
		return f, c
	}
	for _, tc := range []struct {
		name string
		list []string
	}{
		// 250 newer unread notifications, all older than the later run, push the CI notification off the pages read.
		{"its notification past the pages read", cat(skipped(100, maxNotificationPages*notificationsPage, between), []string{ci})},
		{"its notification read on GitHub", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, c := made(t)
			moveOn(t, f, c)
			f.mu.Lock()
			f.list = tc.list
			f.mu.Unlock()
			for poll := range 3 {
				pollOnce(t, c)
				if _, read := item(t, c); read {
					t.Fatalf("poll %d: the item of a later failure GitHub never notified of is marked read", poll+1)
				}
			}
		})
	}

	t.Run("read, then a later run", func(t *testing.T) {
		f, c := made(t)
		f.mu.Lock()
		f.list = nil // the user read the notification on GitHub
		f.mu.Unlock()
		for range 3 {
			pollOnce(t, c)
		}
		if _, read := item(t, c); !read {
			t.Fatal("the notification read on GitHub did not mark the item read")
		}
		moveOn(t, f, c)
		if _, read := item(t, c); read {
			t.Fatal("the later failure stays read: the hub would mark it read again on its new Last")
		}
		for poll := range 3 {
			pollOnce(t, c)
			if _, read := item(t, c); read {
				t.Fatalf("poll %d after the later run: marked read again", poll+1)
			}
		}
	})
}
