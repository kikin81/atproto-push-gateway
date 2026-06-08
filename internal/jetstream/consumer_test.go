package jetstream

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestExtractDIDFromURI(t *testing.T) {
	tests := []struct {
		name string
		uri  string
		want string
	}{
		{"standard post URI", "at://did:plc:abc123/app.bsky.feed.post/xyz", "did:plc:abc123"},
		{"like URI", "at://did:plc:user456/app.bsky.feed.like/rkey", "did:plc:user456"},
		{"did:web URI", "at://did:web:example.org/app.bsky.feed.post/abc", "did:web:example.org"},
		{"empty string", "", ""},
		{"no at:// prefix", "https://example.com", ""},
		{"at:// with no path", "at://did:plc:abc", "did:plc:abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractDIDFromURI(tt.uri); got != tt.want {
				t.Errorf("extractDIDFromURI(%q) = %q, want %q", tt.uri, got, tt.want)
			}
		})
	}
}

func TestParseLikeRecord(t *testing.T) {
	raw := json.RawMessage(`{
		"$type": "app.bsky.feed.like",
		"subject": {
			"uri": "at://did:plc:target/app.bsky.feed.post/abc",
			"cid": "bafyreiabc"
		},
		"createdAt": "2026-04-11T00:00:00Z"
	}`)

	var like LikeRecord
	if err := json.Unmarshal(raw, &like); err != nil {
		t.Fatalf("failed to parse like: %v", err)
	}

	targetDID := extractDIDFromURI(like.Subject.URI)
	if targetDID != "did:plc:target" {
		t.Errorf("expected target did:plc:target, got %s", targetDID)
	}
}

func TestParsePostReply(t *testing.T) {
	raw := json.RawMessage(`{
		"$type": "app.bsky.feed.post",
		"text": "Hello!",
		"reply": {
			"parent": {"uri": "at://did:plc:parent/app.bsky.feed.post/xyz", "cid": "bafyxyz"},
			"root": {"uri": "at://did:plc:root/app.bsky.feed.post/abc", "cid": "bafyabc"}
		},
		"createdAt": "2026-04-11T00:00:00Z"
	}`)

	var post PostRecord
	if err := json.Unmarshal(raw, &post); err != nil {
		t.Fatalf("failed to parse post: %v", err)
	}

	if post.Reply == nil {
		t.Fatal("expected reply to be non-nil")
	}

	parentDID := extractDIDFromURI(post.Reply.Parent.URI)
	if parentDID != "did:plc:parent" {
		t.Errorf("expected parent did:plc:parent, got %s", parentDID)
	}
}

func TestParsePostMention(t *testing.T) {
	raw := json.RawMessage(`{
		"$type": "app.bsky.feed.post",
		"text": "Hey @alice!",
		"facets": [{
			"index": {"byteStart": 4, "byteEnd": 10},
			"features": [{
				"$type": "app.bsky.richtext.facet#mention",
				"did": "did:plc:alice"
			}]
		}],
		"createdAt": "2026-04-11T00:00:00Z"
	}`)

	var post PostRecord
	if err := json.Unmarshal(raw, &post); err != nil {
		t.Fatalf("failed to parse post: %v", err)
	}

	if len(post.Facets) != 1 {
		t.Fatalf("expected 1 facet, got %d", len(post.Facets))
	}

	feature := post.Facets[0].Features[0]
	if feature.Type != "app.bsky.richtext.facet#mention" {
		t.Errorf("expected mention facet, got %s", feature.Type)
	}
	if feature.DID != "did:plc:alice" {
		t.Errorf("expected did:plc:alice, got %s", feature.DID)
	}
}

func TestParsePostQuote(t *testing.T) {
	raw := json.RawMessage(`{
		"$type": "app.bsky.feed.post",
		"text": "Check this out",
		"embed": {
			"$type": "app.bsky.embed.record",
			"record": {
				"uri": "at://did:plc:quoted/app.bsky.feed.post/abc",
				"cid": "bafyabc"
			}
		},
		"createdAt": "2026-04-11T00:00:00Z"
	}`)

	var post PostRecord
	if err := json.Unmarshal(raw, &post); err != nil {
		t.Fatalf("failed to parse post: %v", err)
	}

	if post.Embed == nil {
		t.Fatal("expected embed to be non-nil")
	}
	if post.Embed.Type != "app.bsky.embed.record" {
		t.Errorf("expected embed type app.bsky.embed.record, got %s", post.Embed.Type)
	}

	quotedDID := extractDIDFromURI(post.Embed.Record.URI)
	if quotedDID != "did:plc:quoted" {
		t.Errorf("expected did:plc:quoted, got %s", quotedDID)
	}
}

func TestParseFollowRecord(t *testing.T) {
	raw := json.RawMessage(`{
		"$type": "app.bsky.graph.follow",
		"subject": "did:plc:target",
		"createdAt": "2026-04-11T00:00:00Z"
	}`)

	var follow FollowRecord
	if err := json.Unmarshal(raw, &follow); err != nil {
		t.Fatalf("failed to parse follow: %v", err)
	}

	if follow.Subject != "did:plc:target" {
		t.Errorf("expected did:plc:target, got %s", follow.Subject)
	}
}

func TestParseBlockRecord(t *testing.T) {
	raw := json.RawMessage(`{
		"$type": "app.bsky.graph.block",
		"subject": "did:plc:blocked",
		"createdAt": "2026-04-11T00:00:00Z"
	}`)

	var block BlockRecord
	if err := json.Unmarshal(raw, &block); err != nil {
		t.Fatalf("failed to parse block: %v", err)
	}

	if block.Subject != "did:plc:blocked" {
		t.Errorf("expected did:plc:blocked, got %s", block.Subject)
	}
}

func TestParseJetstreamEvent(t *testing.T) {
	raw := `{
		"did": "did:plc:actor",
		"time_us": 1712800000000000,
		"kind": "commit",
		"commit": {
			"rev": "abc",
			"operation": "create",
			"collection": "app.bsky.feed.like",
			"rkey": "xyz",
			"record": {
				"$type": "app.bsky.feed.like",
				"subject": {
					"uri": "at://did:plc:target/app.bsky.feed.post/123",
					"cid": "bafytest"
				}
			}
		}
	}`

	var event Event
	if err := json.Unmarshal([]byte(raw), &event); err != nil {
		t.Fatalf("failed to parse event: %v", err)
	}

	if event.DID != "did:plc:actor" {
		t.Errorf("expected actor did:plc:actor, got %s", event.DID)
	}
	if event.Kind != "commit" {
		t.Errorf("expected kind commit, got %s", event.Kind)
	}
	if event.Commit == nil {
		t.Fatal("expected commit to be non-nil")
	}
	if event.Commit.Collection != "app.bsky.feed.like" {
		t.Errorf("expected collection app.bsky.feed.like, got %s", event.Commit.Collection)
	}
	if event.Commit.Operation != "create" {
		t.Errorf("expected operation create, got %s", event.Commit.Operation)
	}
}

func TestParseTextOnlyPost(t *testing.T) {
	raw := json.RawMessage(`{
		"$type": "app.bsky.feed.post",
		"text": "Just a text post",
		"createdAt": "2026-04-11T00:00:00Z"
	}`)

	var post PostRecord
	if err := json.Unmarshal(raw, &post); err != nil {
		t.Fatalf("failed to parse post: %v", err)
	}

	if post.Reply != nil {
		t.Error("expected reply to be nil for non-reply post")
	}
	if post.Embed != nil {
		t.Error("expected embed to be nil for text-only post")
	}
	if len(post.Facets) != 0 {
		t.Errorf("expected 0 facets, got %d", len(post.Facets))
	}
}

func TestTruncateBody(t *testing.T) {
	if got := truncateBody("  hello  "); got != "hello" {
		t.Errorf("truncateBody trims: got %q", got)
	}
	if got := truncateBody("   "); got != "" {
		t.Errorf("truncateBody blank → empty: got %q", got)
	}
	short := "a short post"
	if got := truncateBody(short); got != short {
		t.Errorf("truncateBody leaves short text: got %q", got)
	}

	long := strings.Repeat("x", maxBodyRunes+50)
	got := truncateBody(long)
	if r := []rune(got); len(r) != maxBodyRunes+1 { // +1 for the ellipsis rune
		t.Errorf("truncateBody length: got %d runes, want %d", len(r), maxBodyRunes+1)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("truncateBody appends ellipsis: got %q", got)
	}

	// Rune safety: multibyte runes must not be split mid-codepoint.
	multibyte := strings.Repeat("é", maxBodyRunes+10)
	if got := truncateBody(multibyte); !utf8.ValidString(got) {
		t.Errorf("truncateBody produced invalid UTF-8: %q", got)
	}
}

func TestNotificationContent(t *testing.T) {
	// Quote with post text → reason line promoted to title, text is body + bodyText.
	title, body, data := notificationContent("quote", "Alice", "alice.bsky.social", "check this out")
	if title != "Alice quoted your post" {
		t.Errorf("quote title: got %q", title)
	}
	if body != "check this out" || data != "check this out" {
		t.Errorf("quote body/data: got body=%q data=%q", body, data)
	}

	// Reply with text.
	if title, body, data := notificationContent("reply", "Bob", "", "nice"); title != "Bob replied to your post" || body != "nice" || data != "nice" {
		t.Errorf("reply: got title=%q body=%q data=%q", title, body, data)
	}

	// Like (no post text) → unchanged template, no bodyText.
	title, body, data = notificationContent("like", "Bob", "bob.bsky.social", "")
	if title != "New like" || body != "Bob liked your post" || data != "" {
		t.Errorf("like: got title=%q body=%q data=%q", title, body, data)
	}

	// Quote with whitespace-only text → falls back to the templated line, no bodyText.
	title, body, data = notificationContent("quote", "Alice", "", "   ")
	if title != "New quote" || body != "Alice quoted your post" || data != "" {
		t.Errorf("blank quote text fallback: got title=%q body=%q data=%q", title, body, data)
	}

	// Long quote text is truncated in both body and bodyText.
	_, body, data = notificationContent("quote", "Alice", "", strings.Repeat("y", maxBodyRunes+50))
	if r := []rune(body); len(r) != maxBodyRunes+1 {
		t.Errorf("long body truncation: got %d runes", len(r))
	}
	if body != data {
		t.Errorf("body and bodyText must match: body=%q data=%q", body, data)
	}
}
