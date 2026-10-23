package jetstream

import "testing"

// The Nubecita Android client reads the data key "bodyText" (PushPayload.bodyText)
// for the expanded notification body. Upstream ships the same text under
// "reasonSubject" and also for like/repost, so the fork keeps bodyText for
// reply/quote/mention only. These tests pin that contract.

func TestBodyText_ReplyQuoteMentionCarryActorPostText(t *testing.T) {
	c, s, sender := newTestConsumer(t)
	s.RegisterToken("did:plc:bob", "android", "tok-bob", "app.nubecita")
	s.RegisterToken("did:plc:carol", "android", "tok-carol", "app.nubecita")
	s.RegisterToken("did:plc:dave", "android", "tok-dave", "app.nubecita")

	c.handleCommit("did:plc:alice", commitEvent("create", "app.bsky.feed.post", "3kco", map[string]interface{}{
		"text": "hey @dave look at this",
		"reply": map[string]interface{}{
			"parent": map[string]string{"uri": "at://did:plc:bob/app.bsky.feed.post/parent"},
			"root":   map[string]string{"uri": "at://did:plc:bob/app.bsky.feed.post/root"},
		},
		"embed": map[string]interface{}{
			"$type":  "app.bsky.embed.record",
			"record": map[string]string{"uri": "at://did:plc:carol/app.bsky.feed.post/quoted"},
		},
		"facets": []map[string]interface{}{
			{"features": []map[string]string{{"$type": "app.bsky.richtext.facet#mention", "did": "did:plc:dave"}}},
		},
	}))

	sent := sender.notifications()
	if len(sent) != 3 {
		t.Fatalf("expected 3 notifications (reply, quote, mention), got %d", len(sent))
	}
	for _, n := range sent {
		reason := n.Data["reason"]
		got, ok := n.Data["bodyText"]
		if !ok || got == "" {
			t.Errorf("%s: bodyText missing from data %+v", reason, n.Data)
			continue
		}
		if got != n.Data["reasonSubject"] {
			t.Errorf("%s: bodyText %q should equal reasonSubject %q", reason, got, n.Data["reasonSubject"])
		}
	}
}

func TestBodyText_EngagementReasonsOmitBodyText(t *testing.T) {
	c, s, sender := newTestConsumer(t)
	s.RegisterToken("did:plc:bob", "android", "tok-bob", "app.nubecita")

	// For like/repost the text handed to sendNotification is the SUBJECT post's
	// (lazy-fetched upstream), not the actor's own post — it must not become bodyText.
	for _, reason := range []string{"like", "repost", "like-via-repost", "repost-via-repost"} {
		c.sendNotification("did:plc:alice", "did:plc:bob", reason,
			"at://did:plc:alice/app.bsky.feed.like/1", "at://did:plc:bob/app.bsky.feed.post/abc", "bob's own post text")
	}

	sent := sender.notifications()
	if len(sent) != 4 {
		t.Fatalf("expected 4 notifications, got %d", len(sent))
	}
	for _, n := range sent {
		if n.Data["reasonSubject"] != "bob's own post text" {
			t.Errorf("%s: upstream reasonSubject should still be set, got %q", n.Data["reason"], n.Data["reasonSubject"])
		}
		if v, ok := n.Data["bodyText"]; ok {
			t.Errorf("%s: bodyText must be omitted for engagement reasons, got %q", n.Data["reason"], v)
		}
	}
}

func TestBodyText_BlankPostTextOmitsBodyText(t *testing.T) {
	c, s, sender := newTestConsumer(t)
	s.RegisterToken("did:plc:bob", "android", "tok-bob", "app.nubecita")

	c.sendNotification("did:plc:alice", "did:plc:bob", "reply",
		"at://did:plc:alice/app.bsky.feed.post/1", "at://did:plc:bob/app.bsky.feed.post/p", "")

	sent := sender.notifications()
	if len(sent) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(sent))
	}
	if v, ok := sent[0].Data["bodyText"]; ok {
		t.Errorf("blank post text must not produce bodyText, got %q", v)
	}
}
