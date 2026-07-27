package graph

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func collectionQuery(top int) url.Values {
	if top <= 0 {
		return nil
	}
	query := url.Values{}
	query.Set("$top", strconv.Itoa(top))
	return query
}

// SearchMessages performs a Graph full-text search over Teams messages.
func (c *Client) SearchMessages(ctx context.Context, query string, from, size int) (*SearchResponse, error) {
	request := SearchRequest{Requests: []SearchRequestItem{{
		EntityTypes: []string{"chatMessage"},
		Query:       SearchQuery{QueryString: query},
		From:        from,
		Size:        size,
	}}}
	var response SearchResponse
	if err := c.doJSON(ctx, http.MethodPost, "search/query", nil, request, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetChannelMessage returns a specific root channel message.
func (c *Client) GetChannelMessage(ctx context.Context, teamID, channelID, messageID string) (*ChatMessage, error) {
	ref := "teams/" + url.PathEscape(teamID) + "/channels/" + url.PathEscape(channelID) + "/messages/" + url.PathEscape(messageID)
	var message ChatMessage
	if err := c.doJSON(ctx, http.MethodGet, ref, nil, nil, &message); err != nil {
		return nil, err
	}
	return &message, nil
}

// GetChatMessage returns a specific message in an existing chat.
func (c *Client) GetChatMessage(ctx context.Context, chatID, messageID string) (*ChatMessage, error) {
	ref := "chats/" + url.PathEscape(chatID) + "/messages/" + url.PathEscape(messageID)
	var message ChatMessage
	if err := c.doJSON(ctx, http.MethodGet, ref, nil, nil, &message); err != nil {
		return nil, err
	}
	return &message, nil
}

// GetChat returns a specific one-on-one, group, or meeting chat.
func (c *Client) GetChat(ctx context.Context, chatID string) (*Chat, error) {
	query := url.Values{}
	query.Set("$expand", "members($select=id,displayName,roles,userId,email,tenantId)")
	ref := "chats/" + url.PathEscape(chatID)
	var chat Chat
	if err := c.doJSON(ctx, http.MethodGet, ref, query, nil, &chat); err != nil {
		return nil, err
	}
	return &chat, nil
}

// ListChannelMessages returns root messages in a channel, excluding replies.
func (c *Client) ListChannelMessages(ctx context.Context, teamID, channelID, nextLink string, top int) (*Page[ChatMessage], error) {
	initialRef := "teams/" + url.PathEscape(teamID) + "/channels/" + url.PathEscape(channelID) + "/messages"
	ref, err := c.pageRef(nextLink, initialRef)
	if err != nil {
		return nil, err
	}
	var query url.Values
	if nextLink == "" {
		query = collectionQuery(top)
	}
	var page Page[ChatMessage]
	if err := c.doJSON(ctx, http.MethodGet, ref, query, nil, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

// ListChannelMessageReplies returns replies to a root channel message.
func (c *Client) ListChannelMessageReplies(ctx context.Context, teamID, channelID, messageID, nextLink string, top int) (*Page[ChatMessage], error) {
	initialRef := "teams/" + url.PathEscape(teamID) + "/channels/" + url.PathEscape(channelID) + "/messages/" + url.PathEscape(messageID) + "/replies"
	ref, err := c.pageRef(nextLink, initialRef)
	if err != nil {
		return nil, err
	}
	var query url.Values
	if nextLink == "" {
		query = collectionQuery(top)
	}
	var page Page[ChatMessage]
	if err := c.doJSON(ctx, http.MethodGet, ref, query, nil, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

// ListChats returns chats belonging to the delegated or configured user.
func (c *Client) ListChats(ctx context.Context, nextLink string, top int) (*Page[Chat], error) {
	initialRef := c.userPath("chats")
	ref, err := c.pageRef(nextLink, initialRef)
	if err != nil {
		return nil, err
	}
	var query url.Values
	if nextLink == "" {
		query = collectionQuery(top)
		if query == nil {
			query = url.Values{}
		}
		query.Set("$expand", "members($select=id,displayName,roles,userId,email,tenantId)")
	}
	var page Page[Chat]
	if err := c.doJSON(ctx, http.MethodGet, ref, query, nil, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

// ListChatMessages returns messages in a one-on-one, group, or meeting chat.
func (c *Client) ListChatMessages(ctx context.Context, chatID, nextLink string, top int) (*Page[ChatMessage], error) {
	initialRef := "chats/" + url.PathEscape(chatID) + "/messages"
	ref, err := c.pageRef(nextLink, initialRef)
	if err != nil {
		return nil, err
	}
	var query url.Values
	if nextLink == "" {
		query = collectionQuery(top)
	}
	var page Page[ChatMessage]
	if err := c.doJSON(ctx, http.MethodGet, ref, query, nil, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

// ListChatMembers returns members in a one-on-one, group, or meeting chat.
func (c *Client) ListChatMembers(ctx context.Context, chatID, nextLink string, top int) (*Page[ConversationMember], error) {
	initialRef := "chats/" + url.PathEscape(chatID) + "/members"
	ref, err := c.pageRef(nextLink, initialRef)
	if err != nil {
		return nil, err
	}
	var query url.Values
	if nextLink == "" {
		query = collectionQuery(top)
	}
	var page Page[ConversationMember]
	if err := c.doJSON(ctx, http.MethodGet, ref, query, nil, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

type createChatMember struct {
	ODataType string   `json:"@odata.type"`
	Roles     []string `json:"roles"`
	UserBind  string   `json:"user@odata.bind"`
}

type createChatRequest struct {
	ChatType string             `json:"chatType"`
	Topic    string             `json:"topic,omitempty"`
	Members  []createChatMember `json:"members"`
}

// CreateChat creates a one-on-one or group chat with the provided members.
func (c *Client) CreateChat(ctx context.Context, chatType, topic string, participantUserIDs []string) (*Chat, error) {
	request := createChatRequest{ChatType: chatType, Topic: topic, Members: make([]createChatMember, len(participantUserIDs))}
	for i, userID := range participantUserIDs {
		request.Members[i] = createChatMember{
			ODataType: "#microsoft.graph.aadUserConversationMember",
			Roles:     []string{},
			UserBind:  c.userBindURL(userID),
		}
	}
	var chat Chat
	if err := c.doJSON(ctx, http.MethodPost, "chats", nil, request, &chat); err != nil {
		return nil, err
	}
	return &chat, nil
}

// userBindURL builds an OData entity reference. Single quotes are doubled to
// escape them inside the OData string literal.
func (c *Client) userBindURL(userID string) string {
	base := strings.TrimSuffix(c.baseURL.String(), "/")
	return base + "/users('" + url.PathEscape(strings.ReplaceAll(userID, "'", "''")) + "')"
}

type sendMessageRequest struct {
	Body     ItemBody             `json:"body"`
	Mentions []ChatMessageMention `json:"mentions,omitempty"`
}

// SendChannelMessage creates a new root message in a channel.
func (c *Client) SendChannelMessage(ctx context.Context, teamID, channelID, contentType, content string, mentions []ChatMessageMention) (*ChatMessage, error) {
	ref := "teams/" + url.PathEscape(teamID) + "/channels/" + url.PathEscape(channelID) + "/messages"
	return c.sendMessage(ctx, ref, contentType, content, mentions)
}

// ReplyToChannelMessage replies to a root channel message.
func (c *Client) ReplyToChannelMessage(ctx context.Context, teamID, channelID, messageID, contentType, content string, mentions []ChatMessageMention) (*ChatMessage, error) {
	ref := "teams/" + url.PathEscape(teamID) + "/channels/" + url.PathEscape(channelID) + "/messages/" + url.PathEscape(messageID) + "/replies"
	return c.sendMessage(ctx, ref, contentType, content, mentions)
}

// SendChatMessage sends a message to an existing chat.
func (c *Client) SendChatMessage(ctx context.Context, chatID, contentType, content string, mentions []ChatMessageMention) (*ChatMessage, error) {
	ref := "chats/" + url.PathEscape(chatID) + "/messages"
	return c.sendMessage(ctx, ref, contentType, content, mentions)
}

// UpdateChannelMessage updates an existing root channel message.
func (c *Client) UpdateChannelMessage(ctx context.Context, teamID, channelID, messageID, contentType, content string, mentions []ChatMessageMention) (*ChatMessage, error) {
	ref := "teams/" + url.PathEscape(teamID) + "/channels/" + url.PathEscape(channelID) + "/messages/" + url.PathEscape(messageID)
	return c.updateMessage(ctx, ref, messageID, contentType, content, mentions)
}

// UpdateChatMessage updates an existing message in a chat.
func (c *Client) UpdateChatMessage(ctx context.Context, chatID, messageID, contentType, content string, mentions []ChatMessageMention) (*ChatMessage, error) {
	ref := "chats/" + url.PathEscape(chatID) + "/messages/" + url.PathEscape(messageID)
	return c.updateMessage(ctx, ref, messageID, contentType, content, mentions)
}

// DeleteChannelMessage soft-deletes an existing root channel message.
func (c *Client) DeleteChannelMessage(ctx context.Context, teamID, channelID, messageID string) error {
	ref := "teams/" + url.PathEscape(teamID) + "/channels/" + url.PathEscape(channelID) + "/messages/" + url.PathEscape(messageID) + "/softDelete"
	return c.softDeleteMessage(ctx, ref)
}

// DeleteChatMessage soft-deletes an existing message in a chat.
func (c *Client) DeleteChatMessage(ctx context.Context, chatID, messageID string) error {
	ref := "chats/" + url.PathEscape(chatID) + "/messages/" + url.PathEscape(messageID) + "/softDelete"
	return c.softDeleteMessage(ctx, ref)
}

func (c *Client) sendMessage(ctx context.Context, ref, contentType, content string, mentions []ChatMessageMention) (*ChatMessage, error) {
	if contentType == "" {
		contentType = "text"
	}
	request := sendMessageRequest{Body: ItemBody{ContentType: contentType, Content: content}, Mentions: mentions}
	var message ChatMessage
	if err := c.doJSON(ctx, http.MethodPost, ref, nil, request, &message); err != nil {
		return nil, err
	}
	return &message, nil
}

func (c *Client) updateMessage(ctx context.Context, ref, messageID, contentType, content string, mentions []ChatMessageMention) (*ChatMessage, error) {
	if contentType == "" {
		contentType = "text"
	}
	request := sendMessageRequest{Body: ItemBody{ContentType: contentType, Content: content}, Mentions: mentions}
	var message ChatMessage
	if err := c.doJSON(ctx, http.MethodPatch, ref, nil, request, &message); err != nil {
		return nil, err
	}
	if message.ID == "" {
		message.ID = messageID
	}
	return &message, nil
}

func (c *Client) softDeleteMessage(ctx context.Context, ref string) error {
	return c.doJSON(ctx, http.MethodPost, ref, nil, nil, nil)
}
