package graph

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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
	teams, err := client.ListJoinedTeams(t.Context(), "")
	if err != nil || len(teams.Value) != 1 || teams.Value[0].ID != "t1" {
		t.Fatalf("ListJoinedTeams() = %+v, %v", teams, err)
	}
}

func TestListJoinedTeamsPagination(t *testing.T) {
	var baseURL string
	client := newTestClient(t, "", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1.0/me/joinedTeams" && r.URL.Query().Get("$skiptoken") == "":
			_ = json.NewEncoder(w).Encode(Page[Team]{
				Value:    []Team{{ID: "t1"}},
				NextLink: baseURL + "/v1.0/me/joinedTeams?$skiptoken=opaque",
			})
		case r.URL.Path == "/v1.0/me/joinedTeams" && r.URL.Query().Get("$skiptoken") == "opaque":
			_ = json.NewEncoder(w).Encode(Page[Team]{Value: []Team{{ID: "t2"}}})
		default:
			t.Fatalf("unexpected request: %s", r.URL.String())
		}
	})
	baseURL = client.baseURL.Scheme + "://" + client.baseURL.Host

	first, err := client.ListJoinedTeams(t.Context(), "")
	if err != nil || first.NextLink == "" {
		t.Fatalf("first ListJoinedTeams() = %+v, %v", first, err)
	}
	second, err := client.ListJoinedTeams(t.Context(), first.NextLink)
	if err != nil || len(second.Value) != 1 || second.Value[0].ID != "t2" {
		t.Fatalf("second ListJoinedTeams() = %+v, %v", second, err)
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

func TestListTeamMembersPagination(t *testing.T) {
	var baseURL string
	client := newTestClient(t, "", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1.0/teams/team-1/members" && r.URL.Query().Get("$skiptoken") == "":
			if got := r.URL.Query().Get("$top"); got != "25" {
				t.Errorf("$top = %q, want 25", got)
			}
			_ = json.NewEncoder(w).Encode(Page[ConversationMember]{
				Value:    []ConversationMember{{ID: "member-1", UserID: "user-1", DisplayName: "Ada"}},
				NextLink: baseURL + "/v1.0/teams/team-1/members?$skiptoken=opaque",
			})
		case r.URL.Path == "/v1.0/teams/team-1/members" && r.URL.Query().Get("$skiptoken") == "opaque":
			if r.URL.Query().Get("$top") != "" {
				t.Error("next-link request should not add or replace query parameters")
			}
			_ = json.NewEncoder(w).Encode(Page[ConversationMember]{Value: []ConversationMember{{ID: "member-2", UserID: "user-2", DisplayName: "Grace"}}})
		default:
			t.Fatalf("unexpected request: %s", r.URL.String())
		}
	})
	baseURL = client.baseURL.Scheme + "://" + client.baseURL.Host

	first, err := client.ListTeamMembers(t.Context(), "team-1", "", 25)
	if err != nil || first.NextLink == "" || len(first.Value) != 1 || first.Value[0].ID != "member-1" {
		t.Fatalf("first ListTeamMembers() = %+v, %v", first, err)
	}
	second, err := client.ListTeamMembers(t.Context(), "team-1", first.NextLink, 50)
	if err != nil || len(second.Value) != 1 || second.Value[0].ID != "member-2" {
		t.Fatalf("second ListTeamMembers() = %+v, %v", second, err)
	}
}

func TestListUsersPaginationAndFilter(t *testing.T) {
	var baseURL string
	client := newTestClient(t, "", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1.0/users" && r.URL.Query().Get("$skiptoken") == "":
			if got := r.URL.Query().Get("$top"); got != "20" {
				t.Errorf("$top = %q, want 20", got)
			}
			if got := r.URL.Query().Get("$select"); got != "id,displayName,userPrincipalName,mail" {
				t.Errorf("$select = %q", got)
			}
			if got := r.URL.Query().Get("$filter"); !strings.Contains(got, "startswith(displayName,'ada')") {
				t.Errorf("$filter = %q", got)
			}
			_ = json.NewEncoder(w).Encode(Page[User]{
				Value:    []User{{ID: "u1", DisplayName: "Ada", UserPrincipalName: "ada@example.com"}},
				NextLink: baseURL + "/v1.0/users?$skiptoken=opaque",
			})
		case r.URL.Path == "/v1.0/users" && r.URL.Query().Get("$skiptoken") == "opaque":
			if r.URL.Query().Get("$filter") != "" || r.URL.Query().Get("$top") != "" || r.URL.Query().Get("$select") != "" {
				t.Error("next-link request should not add or replace query parameters")
			}
			_ = json.NewEncoder(w).Encode(Page[User]{Value: []User{{ID: "u2", DisplayName: "Grace", UserPrincipalName: "grace@example.com"}}})
		default:
			t.Fatalf("unexpected request: %s", r.URL.String())
		}
	})
	baseURL = client.baseURL.Scheme + "://" + client.baseURL.Host

	first, err := client.ListUsers(t.Context(), "ada", "", 20)
	if err != nil || first.NextLink == "" || len(first.Value) != 1 || first.Value[0].ID != "u1" {
		t.Fatalf("first ListUsers() = %+v, %v", first, err)
	}
	second, err := client.ListUsers(t.Context(), "ignored-on-next-link", first.NextLink, 50)
	if err != nil || len(second.Value) != 1 || second.Value[0].ID != "u2" {
		t.Fatalf("second ListUsers() = %+v, %v", second, err)
	}
}

func TestGetUserByID(t *testing.T) {
	client := newTestClient(t, "", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1.0/users/ada@example.com" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		if got := r.URL.Query().Get("$select"); got != "id,displayName,userPrincipalName,mail" {
			t.Errorf("$select = %q", got)
		}
		_ = json.NewEncoder(w).Encode(User{ID: "u1", DisplayName: "Ada", UserPrincipalName: "ada@example.com", Mail: "ada@example.com"})
	})

	user, err := client.GetUserByID(t.Context(), "ada@example.com")
	if err != nil {
		t.Fatalf("GetUserByID() error = %v", err)
	}
	if user.ID != "u1" || user.UserPrincipalName != "ada@example.com" {
		t.Fatalf("GetUserByID() = %+v", user)
	}
}

func TestGetUserByIDValidation(t *testing.T) {
	client := newTestClient(t, "", func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("HTTP server should not be called")
	})
	if _, err := client.GetUserByID(t.Context(), "   "); err == nil {
		t.Fatal("GetUserByID() accepted empty identifier")
	}
}

func TestGetMessagesByID(t *testing.T) {
	client := newTestClient(t, "", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1.0/teams/team-1/channels/channel-1/messages/message-1":
			_ = json.NewEncoder(w).Encode(ChatMessage{ID: "message-1", Body: ItemBody{ContentType: "text", Content: "channel message"}})
		case "/v1.0/chats/chat-1/messages/message-2":
			_ = json.NewEncoder(w).Encode(ChatMessage{ID: "message-2", Body: ItemBody{ContentType: "text", Content: "chat message"}})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	channelMessage, err := client.GetChannelMessage(t.Context(), "team-1", "channel-1", "message-1")
	if err != nil || channelMessage.ID != "message-1" {
		t.Fatalf("GetChannelMessage() = %+v, %v", channelMessage, err)
	}
	chatMessage, err := client.GetChatMessage(t.Context(), "chat-1", "message-2")
	if err != nil || chatMessage.ID != "message-2" {
		t.Fatalf("GetChatMessage() = %+v, %v", chatMessage, err)
	}
}

func TestGetChannelAndChatByID(t *testing.T) {
	client := newTestClient(t, "", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1.0/teams/team-1/channels/channel-1":
			if got := r.URL.Query().Get("$select"); got != "id,displayName,description,membershipType,webUrl" {
				t.Errorf("$select = %q", got)
			}
			_ = json.NewEncoder(w).Encode(Channel{ID: "channel-1", DisplayName: "General"})
		case "/v1.0/chats/chat-1":
			if got := r.URL.Query().Get("$expand"); got != "members($select=id,displayName,roles,userId,email,tenantId)" {
				t.Errorf("$expand = %q", got)
			}
			_ = json.NewEncoder(w).Encode(Chat{ID: "chat-1", ChatType: "group", Members: []ConversationMember{{ID: "m1", DisplayName: "Ada"}}})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	channel, err := client.GetChannel(t.Context(), "team-1", "channel-1")
	if err != nil || channel.ID != "channel-1" {
		t.Fatalf("GetChannel() = %+v, %v", channel, err)
	}
	chat, err := client.GetChat(t.Context(), "chat-1")
	if err != nil || chat.ID != "chat-1" || len(chat.Members) != 1 {
		t.Fatalf("GetChat() = %+v, %v", chat, err)
	}
}

func TestSearchMessages(t *testing.T) {
	client := newTestClient(t, "", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1.0/search/query" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		var req SearchRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if len(req.Requests) != 1 || len(req.Requests[0].EntityTypes) != 1 || req.Requests[0].EntityTypes[0] != "chatMessage" {
			t.Fatalf("unexpected search request: %+v", req)
		}
		if req.Requests[0].Query.QueryString != "error budget" || req.Requests[0].From != 10 || req.Requests[0].Size != 5 {
			t.Fatalf("unexpected search request parameters: %+v", req.Requests[0])
		}
		_ = json.NewEncoder(w).Encode(SearchResponse{Value: []SearchResponseItem{{
			HitsContainers: []SearchHitsContainer{{
				Total:                1,
				MoreResultsAvailable: true,
				Hits: []SearchHit{{
					HitID:    "h1",
					Resource: ChatMessage{ID: "message-1", Body: ItemBody{ContentType: "text", Content: "error budget"}},
				}},
			}},
		}}})
	})

	response, err := client.SearchMessages(t.Context(), "error budget", 10, 5)
	if err != nil {
		t.Fatalf("SearchMessages() error = %v", err)
	}
	if len(response.Value) != 1 || len(response.Value[0].HitsContainers) != 1 || len(response.Value[0].HitsContainers[0].Hits) != 1 {
		t.Fatalf("SearchMessages() response = %+v", response)
	}
}

func TestListChatsPaginationAndMessages(t *testing.T) {
	var baseURL string
	requestCount := 0
	client := newTestClient(t, "", func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		switch {
		case r.URL.Path == "/v1.0/me/chats" && r.URL.Query().Get("$skiptoken") == "":
			if got := r.URL.Query().Get("$expand"); got != "members($select=id,displayName,roles,userId,email,tenantId)" {
				t.Errorf("$expand = %q", got)
			}
			_ = json.NewEncoder(w).Encode(Page[Chat]{
				Value: []Chat{{
					ID: "chat-1", ChatType: "group",
					Members: []ConversationMember{{
						ID: "member-1", UserID: "user-1", DisplayName: "Ada Lovelace", Email: "ada@example.com",
						Roles: []string{"owner"}, TenantID: "tenant-1", ODataType: "#microsoft.graph.aadUserConversationMember",
					}},
				}},
				NextLink: baseURL + "/v1.0/me/chats?$skiptoken=opaque",
			})
		case r.URL.Path == "/v1.0/me/chats" && r.URL.Query().Get("$skiptoken") == "opaque":
			if r.URL.Query().Get("$top") != "" {
				t.Error("next-link request should not add or replace query parameters")
			}
			if r.URL.Query().Get("$expand") != "" {
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
	if len(first.Value) != 1 || len(first.Value[0].Members) != 1 || first.Value[0].Members[0].DisplayName != "Ada Lovelace" {
		t.Fatalf("first page members = %+v", first.Value)
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

func TestListChatMembersPagination(t *testing.T) {
	var baseURL string
	client := newTestClient(t, "", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1.0/chats/chat-1/members" && r.URL.Query().Get("$skiptoken") == "":
			if got := r.URL.Query().Get("$top"); got != "20" {
				t.Errorf("$top = %q, want 20", got)
			}
			_ = json.NewEncoder(w).Encode(Page[ConversationMember]{
				Value:    []ConversationMember{{ID: "member-1", UserID: "user-1", DisplayName: "Ada"}},
				NextLink: baseURL + "/v1.0/chats/chat-1/members?$skiptoken=opaque",
			})
		case r.URL.Path == "/v1.0/chats/chat-1/members" && r.URL.Query().Get("$skiptoken") == "opaque":
			if r.URL.Query().Get("$top") != "" {
				t.Error("next-link request should not add or replace query parameters")
			}
			_ = json.NewEncoder(w).Encode(Page[ConversationMember]{Value: []ConversationMember{{ID: "member-2", UserID: "user-2", DisplayName: "Grace"}}})
		default:
			t.Fatalf("unexpected request: %s", r.URL.String())
		}
	})
	baseURL = client.baseURL.Scheme + "://" + client.baseURL.Host

	first, err := client.ListChatMembers(t.Context(), "chat-1", "", 20)
	if err != nil || first.NextLink == "" || len(first.Value) != 1 || first.Value[0].ID != "member-1" {
		t.Fatalf("first ListChatMembers() = %+v, %v", first, err)
	}
	second, err := client.ListChatMembers(t.Context(), "chat-1", first.NextLink, 50)
	if err != nil || len(second.Value) != 1 || second.Value[0].ID != "member-2" {
		t.Fatalf("second ListChatMembers() = %+v, %v", second, err)
	}
}

func TestSendMessages(t *testing.T) {
	requestCount := 0
	client := newTestClient(t, "", func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		var body sendMessageRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if requestCount == 1 {
			if len(body.Mentions) != 1 || body.Mentions[0].Mentioned.User == nil || body.Mentions[0].Mentioned.User.ID != "user-1" {
				t.Errorf("mentions = %+v", body.Mentions)
			}
		} else if len(body.Mentions) != 0 {
			t.Errorf("mentions = %+v, want empty", body.Mentions)
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
			return client.SendChannelMessage(t.Context(), "t", "c", "html", "<b>Hello</b>", []ChatMessageMention{{
				ID: 0, MentionText: "Ada", Mentioned: MentionedIdentitySet{User: &Identity{ID: "user-1", DisplayName: "Ada"}},
			}})
		}},
		{"reply", func() (*ChatMessage, error) {
			return client.ReplyToChannelMessage(t.Context(), "t", "c", "m", "html", "<b>Hello</b>", nil)
		}},
		{"chat", func() (*ChatMessage, error) {
			return client.SendChatMessage(t.Context(), "chat", "html", "<b>Hello</b>", nil)
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

func TestUpdateMessages(t *testing.T) {
	requestCount := 0
	client := newTestClient(t, "", func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.Method != http.MethodPatch {
			t.Fatalf("method = %s, want PATCH", r.Method)
		}
		var body sendMessageRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.Body.ContentType != "html" || body.Body.Content != "<b>Updated</b>" {
			t.Fatalf("body = %+v", body)
		}
		if requestCount == 1 {
			_ = json.NewEncoder(w).Encode(ChatMessage{ID: "channel-message-1", Body: body.Body})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	channelUpdated, err := client.UpdateChannelMessage(t.Context(), "team-1", "channel-1", "channel-message-1", "html", "<b>Updated</b>", nil)
	if err != nil || channelUpdated.ID != "channel-message-1" {
		t.Fatalf("UpdateChannelMessage() = %+v, %v", channelUpdated, err)
	}
	chatUpdated, err := client.UpdateChatMessage(t.Context(), "chat-1", "chat-message-2", "html", "<b>Updated</b>", nil)
	if err != nil || chatUpdated.ID != "chat-message-2" {
		t.Fatalf("UpdateChatMessage() = %+v, %v", chatUpdated, err)
	}
}

func TestDeleteMessages(t *testing.T) {
	requestCount := 0
	client := newTestClient(t, "", func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		switch requestCount {
		case 1:
			if r.URL.Path != "/v1.0/teams/team-1/channels/channel-1/messages/message-1/softDelete" {
				t.Fatalf("path = %s", r.URL.Path)
			}
		case 2:
			if r.URL.Path != "/v1.0/chats/chat-1/messages/message-2/softDelete" {
				t.Fatalf("path = %s", r.URL.Path)
			}
		default:
			t.Fatalf("unexpected request count: %d", requestCount)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	if err := client.DeleteChannelMessage(t.Context(), "team-1", "channel-1", "message-1"); err != nil {
		t.Fatalf("DeleteChannelMessage() error = %v", err)
	}
	if err := client.DeleteChatMessage(t.Context(), "chat-1", "message-2"); err != nil {
		t.Fatalf("DeleteChatMessage() error = %v", err)
	}
}

func TestCreateChat(t *testing.T) {
	client := newTestClient(t, "", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1.0/chats" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		var body createChatRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.ChatType != "group" || body.Topic != "Incident response" {
			t.Fatalf("unexpected chat metadata: %+v", body)
		}
		if len(body.Members) != 2 {
			t.Fatalf("members = %+v, want 2", body.Members)
		}
		if body.Members[0].ODataType != "#microsoft.graph.aadUserConversationMember" {
			t.Fatalf("unexpected odata type: %+v", body.Members[0])
		}
		if !strings.Contains(body.Members[0].UserBind, "/v1.0/users('user-1')") {
			t.Fatalf("unexpected member bind: %q", body.Members[0].UserBind)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(Chat{ID: "chat-42", ChatType: "group", Topic: "Incident response", WebURL: "https://teams.microsoft.com/l/chat/42"})
	})

	chat, err := client.CreateChat(t.Context(), "group", "Incident response", []string{"user-1", "user-2"})
	if err != nil {
		t.Fatalf("CreateChat() error = %v", err)
	}
	if chat.ID != "chat-42" || chat.ChatType != "group" {
		t.Fatalf("CreateChat() = %+v", chat)
	}
}

func TestAPIError(t *testing.T) {
	requestCount := 0
	client := newTestClient(t, "", func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("request-id", "request-123")
		w.Header().Set("Retry-After", "5")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": "TooManyRequests", "message": "slow down"}})
	})
	client.wait = func(_ context.Context, _ time.Duration) error { return nil }

	_, err := client.ListChats(t.Context(), "", 20)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %T %v, want *APIError", err, err)
	}
	if apiErr.StatusCode != 429 || apiErr.Code != "TooManyRequests" || apiErr.RequestID != "request-123" || apiErr.RetryAfter != "5" {
		t.Errorf("APIError = %+v", apiErr)
	}
	if requestCount != client.retries {
		t.Errorf("request count = %d, want %d", requestCount, client.retries)
	}
}

func TestRetryDelay(t *testing.T) {
	for _, tc := range []struct {
		name       string
		retryAfter string
		attempt    int
		want       time.Duration
	}{
		{name: "seconds", retryAfter: "1", attempt: 1, want: time.Second},
		{name: "caps long retry-after", retryAfter: "3600", attempt: 1, want: retryMaxDelay},
		{name: "negative", retryAfter: "-5", attempt: 1, want: 0},
		{name: "rejects duration syntax", retryAfter: "1m", attempt: 1, want: retryBaseDelay},
		{name: "exponential backoff", retryAfter: "", attempt: 2, want: retryBaseDelay * 2},
		{name: "backoff capped", retryAfter: "", attempt: 10, want: retryMaxDelay},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := retryDelay(tc.retryAfter, tc.attempt); got != tc.want {
				t.Errorf("retryDelay(%q, %d) = %v, want %v", tc.retryAfter, tc.attempt, got, tc.want)
			}
		})
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
