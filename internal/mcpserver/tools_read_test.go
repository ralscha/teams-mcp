package mcpserver

import (
	"testing"

	"teams-mcp/internal/graph"
)

func TestMessageToSummaryMapsChannelIdentityAndSender(t *testing.T) {
	message := graph.ChatMessage{
		ID:                   "message-1",
		ReplyToID:            "root-1",
		MessageType:          "message",
		CreatedDateTime:      "2026-07-27T10:00:00Z",
		LastModifiedDateTime: "2026-07-27T10:01:00Z",
		Body:                 graph.ItemBody{ContentType: "text", Content: "hello"},
		WebURL:               "https://teams.microsoft.com/l/message/1",
		From:                 &graph.ChatMessageFrom{User: &graph.Identity{ID: "user-1", DisplayName: "Ada"}},
		ChannelIdentity:      &graph.ChannelIdentity{TeamID: "team-1", ChannelID: "channel-1"},
		Mentions: []graph.ChatMessageMention{{
			ID: 0, MentionText: "Ada", Mentioned: graph.MentionedIdentitySet{User: &graph.Identity{ID: "user-1", DisplayName: "Ada"}},
		}},
		Attachments: []graph.ChatMessageAttachment{{
			ID: "att-1", Name: "file.txt", ContentType: "text/plain", ContentURL: "https://example.test/file.txt",
		}},
	}

	summary := messageToSummary(message)

	if summary.ID != "message-1" || summary.ReplyToID != "root-1" {
		t.Fatalf("unexpected message identity mapping: %+v", summary)
	}
	if summary.SenderType != "user" || summary.SenderID != "user-1" || summary.SenderName != "Ada" {
		t.Fatalf("unexpected sender mapping: %+v", summary)
	}
	if summary.ChannelIdentity == nil {
		t.Fatalf("channel_identity is nil: %+v", summary)
	}
	if summary.ChannelIdentity.TeamID != "team-1" || summary.ChannelIdentity.ChannelID != "channel-1" {
		t.Fatalf("unexpected channel identity mapping: %+v", summary.ChannelIdentity)
	}
	if len(summary.Mentions) != 1 {
		t.Fatalf("unexpected mentions mapping: %+v", summary.Mentions)
	}
	if summary.Mentions[0].Type != "user" || summary.Mentions[0].MentionedID != "user-1" || summary.Mentions[0].DisplayName != "Ada" {
		t.Fatalf("unexpected mention mapping: %+v", summary.Mentions[0])
	}
	if len(summary.Attachments) != 1 || summary.Attachments[0].ID != "att-1" {
		t.Fatalf("unexpected attachments mapping: %+v", summary.Attachments)
	}
}

func TestMessageToSummaryOmitsEmptyChannelIdentity(t *testing.T) {
	summary := messageToSummary(graph.ChatMessage{ID: "message-2", Body: graph.ItemBody{ContentType: "text", Content: "hi"}})
	if summary.ChannelIdentity != nil {
		t.Fatalf("channel_identity should be omitted for chat messages: %+v", summary.ChannelIdentity)
	}
}
