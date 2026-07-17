package graph

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestClient(t *testing.T, userID string, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := NewClient(server.URL+"/v1.0/", userID, server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	return client
}

func TestGetUserAndJoinedTeams(t *testing.T) {
	client := newTestClient(t, "user@example.com", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1.0/users/user@example.com":
			if r.URL.Query().Get("$select") == "" {
				t.Error("GetUser request omitted $select")
			}
			_ = json.NewEncoder(w).Encode(User{ID: "u1", DisplayName: "Ada", UserPrincipalName: "user@example.com"})
		case "/v1.0/users/user@example.com/joinedTeams":
			_ = json.NewEncoder(w).Encode(Page[Team]{Value: []Team{{ID: "t1", DisplayName: "Engineering"}}})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	user, err := client.GetUser(t.Context())
	if err != nil || user.ID != "u1" {
		t.Fatalf("GetUser() = %+v, %v", user, err)
	}
	teams, err := client.ListJoinedTeams(t.Context())
	if err != nil || len(teams.Value) != 1 || teams.Value[0].ID != "t1" {
		t.Fatalf("ListJoinedTeams() = %+v, %v", teams, err)
	}
}

func TestListChannelsAndMessages(t *testing.T) {
	client := newTestClient(t, "", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1.0/teams/team-1/channels":
			if got := r.URL.Query().Get("$select"); got != "id,displayName,description,membershipType,webUrl" {
				t.Errorf("$select = %q", got)
			}
			_ = json.NewEncoder(w).Encode(Page[Channel]{Value: []Channel{{ID: "channel-1", DisplayName: "General"}}})
		case "/v1.0/teams/team-1/channels/channel-1/messages":
			if got := r.URL.Query().Get("$top"); got != "25" {
				t.Errorf("$top = %q, want 25", got)
			}
			_ = json.NewEncoder(w).Encode(Page[ChatMessage]{Value: []ChatMessage{{ID: "message-1", Body: ItemBody{ContentType: "text", Content: "hello"}}}})
		case "/v1.0/teams/team-1/channels/channel-1/messages/message-1/replies":
			_ = json.NewEncoder(w).Encode(Page[ChatMessage]{Value: []ChatMessage{{ID: "reply-1", ReplyToID: "message-1"}}})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	channels, err := client.ListChannels(t.Context(), "team-1", "")
	if err != nil || len(channels.Value) != 1 {
		t.Fatalf("ListChannels() = %+v, %v", channels, err)
	}
	messages, err := client.ListChannelMessages(t.Context(), "team-1", "channel-1", "", 25)
	if err != nil || len(messages.Value) != 1 || messages.Value[0].Body.Content != "hello" {
		t.Fatalf("ListChannelMessages() = %+v, %v", messages, err)
	}
	replies, err := client.ListChannelMessageReplies(t.Context(), "team-1", "channel-1", "message-1", "", 20)
	if err != nil || len(replies.Value) != 1 || replies.Value[0].ReplyToID != "message-1" {
		t.Fatalf("ListChannelMessageReplies() = %+v, %v", replies, err)
	}
}

func TestListChatsPaginationAndMessages(t *testing.T) {
	var baseURL string
	requestCount := 0
	client := newTestClient(t, "", func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		switch {
		case r.URL.Path == "/v1.0/me/chats" && r.URL.Query().Get("$skiptoken") == "":
			_ = json.NewEncoder(w).Encode(Page[Chat]{
				Value:    []Chat{{ID: "chat-1", ChatType: "group"}},
				NextLink: baseURL + "/v1.0/me/chats?$skiptoken=opaque",
			})
		case r.URL.Path == "/v1.0/me/chats" && r.URL.Query().Get("$skiptoken") == "opaque":
			if r.URL.Query().Get("$top") != "" {
				t.Error("next-link request should not add or replace query parameters")
			}
			_ = json.NewEncoder(w).Encode(Page[Chat]{Value: []Chat{{ID: "chat-2"}}})
		case r.URL.Path == "/v1.0/chats/chat-1/messages":
			_ = json.NewEncoder(w).Encode(Page[ChatMessage]{Value: []ChatMessage{{ID: "m1"}}})
		default:
			t.Fatalf("unexpected request: %s", r.URL.String())
		}
	})
	baseURL = client.baseURL.Scheme + "://" + client.baseURL.Host

	first, err := client.ListChats(t.Context(), "", 10)
	if err != nil || first.NextLink == "" {
		t.Fatalf("first ListChats() = %+v, %v", first, err)
	}
	second, err := client.ListChats(t.Context(), first.NextLink, 50)
	if err != nil || len(second.Value) != 1 || second.Value[0].ID != "chat-2" {
		t.Fatalf("second ListChats() = %+v, %v", second, err)
	}
	messages, err := client.ListChatMessages(t.Context(), "chat-1", "", 20)
	if err != nil || len(messages.Value) != 1 {
		t.Fatalf("ListChatMessages() = %+v, %v", messages, err)
	}
	if requestCount != 3 {
		t.Errorf("request count = %d, want 3", requestCount)
	}
}

func TestSendMessages(t *testing.T) {
	client := newTestClient(t, "", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		var body sendMessageRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.Body.ContentType != "html" || body.Body.Content != "<b>Hello</b>" {
			t.Errorf("body = %+v", body)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(ChatMessage{ID: "created", Body: body.Body})
	})

	tests := []struct {
		name string
		send func() (*ChatMessage, error)
	}{
		{"channel", func() (*ChatMessage, error) {
			return client.SendChannelMessage(t.Context(), "t", "c", "html", "<b>Hello</b>")
		}},
		{"reply", func() (*ChatMessage, error) {
			return client.ReplyToChannelMessage(t.Context(), "t", "c", "m", "html", "<b>Hello</b>")
		}},
		{"chat", func() (*ChatMessage, error) {
			return client.SendChatMessage(t.Context(), "chat", "html", "<b>Hello</b>")
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			message, err := test.send()
			if err != nil || message.ID != "created" {
				t.Fatalf("send() = %+v, %v", message, err)
			}
		})
	}
}

func TestAPIError(t *testing.T) {
	client := newTestClient(t, "", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("request-id", "request-123")
		w.Header().Set("Retry-After", "5")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": "TooManyRequests", "message": "slow down"}})
	})

	_, err := client.ListChats(t.Context(), "", 20)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %T %v, want *APIError", err, err)
	}
	if apiErr.StatusCode != 429 || apiErr.Code != "TooManyRequests" || apiErr.RequestID != "request-123" || apiErr.RetryAfter != "5" {
		t.Errorf("APIError = %+v", apiErr)
	}
}

func TestRejectsUntrustedNextLink(t *testing.T) {
	client := newTestClient(t, "", func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("HTTP server should not be called")
	})

	for _, nextLink := range []string{
		"https://evil.example/v1.0/chats?$skiptoken=secret",
		strings.Replace(client.baseURL.String(), "/v1.0/", "/beta/", 1) + "chats",
		client.baseURL.String() + "users?$skiptoken=secret",
		"../beta/chats?$skiptoken=secret",
	} {
		if _, err := client.ListChats(t.Context(), nextLink, 20); err == nil {
			t.Errorf("ListChats() accepted untrusted next link %q", nextLink)
		}
	}
}
