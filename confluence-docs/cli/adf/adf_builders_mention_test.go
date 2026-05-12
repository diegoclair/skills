package adf

import (
	"strings"
	"testing"
)

// ---------- mock resolver for tests (no live API calls) ----------

// mockResolver is a UserResolver that returns a fixed mapping.
type mockResolver struct {
	users map[string]string // query → accountId; "" value = not found
}

func newMockResolver(pairs ...string) *mockResolver {
	m := &mockResolver{users: make(map[string]string)}
	for i := 0; i+1 < len(pairs); i += 2 {
		m.users[pairs[i]] = pairs[i+1]
	}
	return m
}

func (m *mockResolver) Resolve(query string) (string, bool) {
	id, found := m.users[query]
	if !found {
		return "", false
	}
	if id == "" {
		return "", false
	}
	return id, true
}

// ---------- renderPropertiesValueWithResolver tests ----------

func TestRenderPropertiesValue_AtMention_Resolves(t *testing.T) {
	resolver := newMockResolver("diegoclair", "abc-123-account")
	out := renderPropertiesValueWithResolver("@diegoclair", resolver)
	if !strings.Contains(out, `ri:account-id="abc-123-account"`) {
		t.Errorf("expected user mention link; got: %s", out)
	}
	if !strings.Contains(out, "<ac:link>") {
		t.Errorf("expected <ac:link> wrapper; got: %s", out)
	}
	if !strings.Contains(out, "<ri:user") {
		t.Errorf("expected <ri:user> element; got: %s", out)
	}
	// Must NOT contain the raw @diegoclair text in the output.
	if strings.Contains(out, "@diegoclair") {
		t.Errorf("resolved mention should not contain raw @handle; got: %s", out)
	}
}

func TestRenderPropertiesValue_AtMention_NotFound(t *testing.T) {
	resolver := newMockResolver() // empty — nothing resolves
	out := renderPropertiesValueWithResolver("@unknownhandle", resolver)
	// Should fall back to plain text (XML-escaped).
	if strings.Contains(out, "<ri:user") {
		t.Errorf("unresolved mention should not produce user link; got: %s", out)
	}
	if !strings.Contains(out, "@unknownhandle") {
		t.Errorf("unresolved mention should keep @handle as plain text; got: %s", out)
	}
}

func TestRenderPropertiesValue_EmailMention_Resolves(t *testing.T) {
	resolver := newMockResolver("d.clair@novapaytech.com", "email-account-456")
	out := renderPropertiesValueWithResolver("d.clair@novapaytech.com", resolver)
	if !strings.Contains(out, `ri:account-id="email-account-456"`) {
		t.Errorf("expected user mention from email; got: %s", out)
	}
	if !strings.Contains(out, "<ri:user") {
		t.Errorf("expected <ri:user>; got: %s", out)
	}
}

func TestRenderPropertiesValue_EmailMention_NotFound(t *testing.T) {
	resolver := newMockResolver()
	out := renderPropertiesValueWithResolver("unknown@example.com", resolver)
	if strings.Contains(out, "<ri:user") {
		t.Errorf("unresolved email should not produce user link; got: %s", out)
	}
	if !strings.Contains(out, "unknown@example.com") {
		t.Errorf("unresolved email should stay as plain text; got: %s", out)
	}
}

func TestRenderPropertiesValue_MultipleHandles(t *testing.T) {
	resolver := newMockResolver(
		"alice", "acc-alice",
		"bob", "acc-bob",
	)
	// Simulate an "owner" value that contains two @mentions.
	out := renderPropertiesValueWithResolver("@alice, @bob", resolver)
	if !strings.Contains(out, `ri:account-id="acc-alice"`) {
		t.Errorf("alice not resolved; got: %s", out)
	}
	if !strings.Contains(out, `ri:account-id="acc-bob"`) {
		t.Errorf("bob not resolved; got: %s", out)
	}
	// The comma/space between mentions should still be present.
	if !strings.Contains(out, ", ") {
		t.Errorf("expected separator between mentions; got: %s", out)
	}
}

func TestRenderPropertiesValue_MixedMentionAndPageLink(t *testing.T) {
	resolver := newMockResolver("diegoclair", "acc-diego")
	// Value has both a [[page link]] and an @handle.
	out := renderPropertiesValueWithResolver("[[Some Page]], @diegoclair", resolver)
	if !strings.Contains(out, `ri:content-title="Some Page"`) {
		t.Errorf("page link missing; got: %s", out)
	}
	if !strings.Contains(out, `ri:account-id="acc-diego"`) {
		t.Errorf("user mention missing; got: %s", out)
	}
}

func TestRenderPropertiesValue_PlainText_Unchanged(t *testing.T) {
	resolver := newMockResolver()
	out := renderPropertiesValueWithResolver("reference", resolver)
	if out != "reference" {
		t.Errorf("plain text should be unchanged; got: %s", out)
	}
}

func TestRenderPropertiesValue_NoopResolver_IsBackwardCompatible(t *testing.T) {
	// renderPropertiesValue (no resolver) must behave exactly as before:
	// @handles stay as plain text.
	out := renderPropertiesValue("@diegoclair")
	if strings.Contains(out, "<ri:user") {
		t.Errorf("noop renderPropertiesValue should not produce user link; got: %s", out)
	}
	if !strings.Contains(out, "@diegoclair") {
		t.Errorf("noop should keep @handle as plain text; got: %s", out)
	}
}

// ---------- PagePropertiesToStorage with resolver ----------

func TestPagePropertiesToStorage_WithResolver(t *testing.T) {
	resolver := newMockResolver("diegoclair", "acc-diego-123")
	entries := []PagePropertiesEntry{
		{Key: "owner", Value: "@diegoclair"},
		{Key: "status", Value: "ativo"},
	}
	out := PagePropertiesToStorage(entries, resolver)
	if !strings.Contains(out, `ri:account-id="acc-diego-123"`) {
		t.Errorf("expected user mention in output; got: %s", out)
	}
	if !strings.Contains(out, "<td>ativo</td>") {
		t.Errorf("plain value 'ativo' missing; got: %s", out)
	}
}

func TestPagePropertiesToStorage_NoResolver_BackwardCompat(t *testing.T) {
	// Calling without resolver (variadic empty) must still work and leave
	// @handle as plain text — existing callers are unaffected.
	entries := []PagePropertiesEntry{
		{Key: "owner", Value: "@diegoclair"},
	}
	out := PagePropertiesToStorage(entries)
	if strings.Contains(out, "<ri:user") {
		t.Errorf("no-resolver call should not produce user link; got: %s", out)
	}
	if !strings.Contains(out, "@diegoclair") {
		t.Errorf("no-resolver call should keep @handle as plain text; got: %s", out)
	}
}

// ---------- XML escaping safety ----------

func TestRenderPropertiesValue_AtMention_AccountIDEscaped(t *testing.T) {
	// Ensures an accountId containing special chars is safely escaped.
	resolver := newMockResolver("danger", `foo"bar<baz>`)
	out := renderPropertiesValueWithResolver("@danger", resolver)
	// Should NOT contain raw unescaped chars.
	if strings.Contains(out, `foo"bar`) {
		t.Errorf("accountId should be XML-escaped in attribute; got: %s", out)
	}
	if strings.Contains(out, `<baz>`) {
		t.Errorf("accountId < > should be escaped; got: %s", out)
	}
}
