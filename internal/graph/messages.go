package graph

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

func collectionQuery(top int) url.Values {
	if top <= 0 {
		return nil
	}
	query := url.Values{}
	query.Set("$top", strconv.Itoa(top))
	return query
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

type sendMessageRequest struct {
	Body ItemBody `json:"body"`
}

// SendChannelMessage creates a new root message in a channel.
func (c *Client) SendChannelMessage(ctx context.Context, teamID, channelID, contentType, content string) (*ChatMessage, error) {
	ref := "teams/" + url.PathEscape(teamID) + "/channels/" + url.PathEscape(channelID) + "/messages"
	return c.sendMessage(ctx, ref, contentType, content)
}

// ReplyToChannelMessage replies to a root channel message.
func (c *Client) ReplyToChannelMessage(ctx context.Context, teamID, channelID, messageID, contentType, content string) (*ChatMessage, error) {
	ref := "teams/" + url.PathEscape(teamID) + "/channels/" + url.PathEscape(channelID) + "/messages/" + url.PathEscape(messageID) + "/replies"
	return c.sendMessage(ctx, ref, contentType, content)
}

// SendChatMessage sends a message to an existing chat.
func (c *Client) SendChatMessage(ctx context.Context, chatID, contentType, content string) (*ChatMessage, error) {
	ref := "chats/" + url.PathEscape(chatID) + "/messages"
	return c.sendMessage(ctx, ref, contentType, content)
}

func (c *Client) sendMessage(ctx context.Context, ref, contentType, content string) (*ChatMessage, error) {
	if contentType == "" {
		contentType = "text"
	}
	request := sendMessageRequest{Body: ItemBody{ContentType: contentType, Content: content}}
	var message ChatMessage
	if err := c.doJSON(ctx, http.MethodPost, ref, nil, request, &message); err != nil {
		return nil, err
	}
	return &message, nil
}
