package mcpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"teams-mcp/internal/config"
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

func TestSearchMessagesValidation(t *testing.T) {
	handler := searchMessages(testGraphClient(t))
	for _, input := range []SearchMessagesInput{{Query: "   "}, {Query: "hello", From: -1}} {
		if _, _, err := handler(t.Context(), nil, input); err == nil {
			t.Errorf("search accepted invalid input: %+v", input)
		}
	}
}

func TestReadToolReplyAndSearchSnippet(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1.0/teams/team/channels/channel/messages/root/replies/reply":
			_ = json.NewEncoder(w).Encode(graph.ChatMessage{
				ID: "reply", ReplyToID: "root", Body: graph.ItemBody{ContentType: "text", Content: "reply body"},
			})
		case "/v1.0/search/query":
			_, _ = w.Write([]byte(`{"value":[{"hitsContainers":[{"total":1,"hits":[{"summary":"matching snippet","resource":{"id":"match"}}]}]}]}`))
		default:
			t.Errorf("unexpected Graph request %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer backend.Close()
	graphClient, err := graph.NewClient(backend.URL+"/v1.0/", "", backend.Client())
	if err != nil {
		t.Fatal(err)
	}
	server := NewServer(&config.Config{Mode: config.ModeReadOnly}, graphClient)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(t.Context(), serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = serverSession.Close() }()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	session, err := client.Connect(t.Context(), clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = session.Close() }()
	for _, tc := range []struct {
		name string
		args map[string]any
	}{
		{"teams_get_channel_message_reply", map[string]any{"team_id": "team", "channel_id": "channel", "message_id": "root", "reply_id": "reply"}},
		{"teams_search_messages", map[string]any{"query": "matching"}},
	} {
		result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: tc.name, Arguments: tc.args})
		if err != nil || result.IsError {
			t.Fatalf("CallTool(%s) = %+v, %v", tc.name, result, err)
		}
		data, err := json.Marshal(result.StructuredContent)
		if err != nil {
			t.Fatal(err)
		}
		if tc.name == "teams_get_channel_message_reply" {
			var reply MessageSummary
			if err := json.Unmarshal(data, &reply); err != nil || reply.ID != "reply" || reply.ReplyToID != "root" || reply.Content != "reply body" {
				t.Errorf("reply result=%s, error=%v", data, err)
			}
		} else {
			var matches SearchMessagesOutput
			if err := json.Unmarshal(data, &matches); err != nil || len(matches.Messages) != 1 || matches.Messages[0].Summary != "matching snippet" {
				t.Errorf("search result=%s, error=%v", data, err)
			}
		}
	}
}

func TestMessageToSummaryOmitsEmptyChannelIdentity(t *testing.T) {
	summary := messageToSummary(graph.ChatMessage{ID: "message-2", Body: graph.ItemBody{ContentType: "text", Content: "hi"}})
	if summary.ChannelIdentity != nil {
		t.Fatalf("channel_identity should be omitted for chat messages: %+v", summary.ChannelIdentity)
	}
}
