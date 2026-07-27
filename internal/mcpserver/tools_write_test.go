package mcpserver

import (
	"reflect"
	"testing"
)

func TestBuildMentions(t *testing.T) {
	mentions, err := buildMentions("html", []OutboundMention{{UserID: "user-1", DisplayName: "Ada"}})
	if err != nil {
		t.Fatalf("buildMentions() error = %v", err)
	}
	if len(mentions) != 1 {
		t.Fatalf("len(mentions) = %d, want 1", len(mentions))
	}
	if mentions[0].ID != 0 || mentions[0].Mentioned.User == nil || mentions[0].Mentioned.User.ID != "user-1" {
		t.Fatalf("mentions[0] = %+v", mentions[0])
	}
}

func TestBuildMentionsValidation(t *testing.T) {
	if _, err := buildMentions("text", []OutboundMention{{UserID: "user-1"}}); err == nil {
		t.Fatal("buildMentions() accepted non-html content type")
	}
	if _, err := buildMentions("html", []OutboundMention{{UserID: "   "}}); err == nil {
		t.Fatal("buildMentions() accepted empty user id")
	}
	mentions, err := buildMentions("html", nil)
	if err != nil {
		t.Fatalf("buildMentions(nil) error = %v", err)
	}
	if len(mentions) != 0 {
		t.Fatalf("len(mentions) = %d, want 0", len(mentions))
	}
}

func TestNormalizeParticipantIDs(t *testing.T) {
	ids, err := normalizeParticipantIDs([]string{" user-1 ", "user-2", "user-1"})
	if err != nil {
		t.Fatalf("normalizeParticipantIDs() error = %v", err)
	}
	if !reflect.DeepEqual(ids, []string{"user-1", "user-2"}) {
		t.Fatalf("normalizeParticipantIDs() = %#v", ids)
	}
}

func TestNormalizeParticipantIDsValidation(t *testing.T) {
	if _, err := normalizeParticipantIDs(nil); err == nil {
		t.Fatal("normalizeParticipantIDs(nil) accepted empty list")
	}
	if _, err := normalizeParticipantIDs([]string{"user-1", "   "}); err == nil {
		t.Fatal("normalizeParticipantIDs() accepted empty id")
	}
}

func TestValidCreateChatType(t *testing.T) {
	if !validCreateChatType("oneOnOne") || !validCreateChatType("group") {
		t.Fatal("validCreateChatType() rejected valid values")
	}
	if validCreateChatType("meeting") {
		t.Fatal("validCreateChatType() accepted unsupported value")
	}
}
