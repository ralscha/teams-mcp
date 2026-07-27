package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"teams-mcp/internal/graph"
)

var nonDestructiveHint = &mcp.ToolAnnotations{DestructiveHint: new(false)}

var destructiveHint = &mcp.ToolAnnotations{DestructiveHint: new(true)}

// SentMessage identifies a message created by a write tool.
type SentMessage struct {
	MessageID       string `json:"message_id" jsonschema:"the created message id"`
	CreatedDateTime string `json:"created_date_time,omitempty" jsonschema:"message creation timestamp"`
	WebURL          string `json:"web_url,omitempty" jsonschema:"the message URL in Microsoft Teams"`
}

// CreatedChat identifies a chat created by teams_create_chat.
type CreatedChat struct {
	ChatID   string `json:"chat_id" jsonschema:"the created chat id"`
	ChatType string `json:"chat_type,omitempty" jsonschema:"oneOnOne or group"`
	Topic    string `json:"topic,omitempty" jsonschema:"the group chat topic, if provided"`
	WebURL   string `json:"web_url,omitempty" jsonschema:"the chat URL in Microsoft Teams"`
}

// DeletedMessage identifies a message soft-deleted by a write tool.
type DeletedMessage struct {
	MessageID string `json:"message_id" jsonschema:"the deleted message id"`
}

// OutboundMention identifies one user mention included in an outbound message.
type OutboundMention struct {
	UserID      string `json:"user_id" jsonschema:"the Entra user id to mention"`
	DisplayName string `json:"display_name,omitempty" jsonschema:"display name used in mention metadata"`
}

func validateMessage(contentType, content string) (string, error) {
	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("content must not be empty")
	}
	if contentType == "" {
		return "text", nil
	}
	if contentType != "text" && contentType != "html" {
		return "", fmt.Errorf("content_type must be %q or %q", "text", "html")
	}
	return contentType, nil
}

func sentMessage(message *graph.ChatMessage) SentMessage {
	return SentMessage{MessageID: message.ID, CreatedDateTime: message.CreatedDateTime, WebURL: message.WebURL}
}

func validCreateChatType(value string) bool {
	return value == "oneOnOne" || value == "group"
}

func normalizeParticipantIDs(raw []string) ([]string, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("participant_user_ids must not be empty")
	}
	out := make([]string, 0, len(raw))
	seen := map[string]bool{}
	for i, userID := range raw {
		id := strings.TrimSpace(userID)
		if id == "" {
			return nil, fmt.Errorf("participant_user_ids[%d] must not be empty", i)
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out, nil
}

// CreateChatInput is the input for teams_create_chat.
type CreateChatInput struct {
	ChatType           string   `json:"chat_type,omitempty" jsonschema:"oneOnOne (default) or group"`
	Topic              string   `json:"topic,omitempty" jsonschema:"group chat topic; ignored for oneOnOne chats"`
	ParticipantUserIDs []string `json:"participant_user_ids" jsonschema:"Entra user ids to include in the new chat"`
}

func createChat(client *graph.Client) mcp.ToolHandlerFor[CreateChatInput, CreatedChat] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in CreateChatInput) (*mcp.CallToolResult, CreatedChat, error) {
		chatType := strings.TrimSpace(in.ChatType)
		if chatType == "" {
			chatType = "oneOnOne"
		}
		if !validCreateChatType(chatType) {
			return nil, CreatedChat{}, fmt.Errorf("chat_type must be %q or %q", "oneOnOne", "group")
		}
		participantUserIDs, err := normalizeParticipantIDs(in.ParticipantUserIDs)
		if err != nil {
			return nil, CreatedChat{}, err
		}
		if chatType == "oneOnOne" && len(participantUserIDs) != 1 {
			return nil, CreatedChat{}, fmt.Errorf("oneOnOne chat requires exactly 1 participant_user_id")
		}
		topic := strings.TrimSpace(in.Topic)
		if chatType == "oneOnOne" {
			topic = ""
		}
		chat, err := client.CreateChat(ctx, chatType, topic, participantUserIDs)
		if err != nil {
			return nil, CreatedChat{}, fmt.Errorf("create chat: %w", err)
		}
		return nil, CreatedChat{ChatID: chat.ID, ChatType: chat.ChatType, Topic: chat.Topic, WebURL: chat.WebURL}, nil
	}
}

func buildMentions(contentType string, mentions []OutboundMention) ([]graph.ChatMessageMention, error) {
	if len(mentions) == 0 {
		return nil, nil
	}
	if contentType != "html" {
		return nil, fmt.Errorf("mentions require content_type %q", "html")
	}
	out := make([]graph.ChatMessageMention, len(mentions))
	for i, mention := range mentions {
		if strings.TrimSpace(mention.UserID) == "" {
			return nil, fmt.Errorf("mentions[%d].user_id must not be empty", i)
		}
		out[i] = graph.ChatMessageMention{
			ID:          i,
			MentionText: mention.DisplayName,
			Mentioned: graph.MentionedIdentitySet{
				User: &graph.Identity{ID: mention.UserID, DisplayName: mention.DisplayName},
			},
		}
	}
	return out, nil
}

// SendChannelMessageInput is the input for teams_send_channel_message.
type SendChannelMessageInput struct {
	TeamID      string            `json:"team_id" jsonschema:"the team id"`
	ChannelID   string            `json:"channel_id" jsonschema:"the channel id"`
	Content     string            `json:"content" jsonschema:"message body to send"`
	ContentType string            `json:"content_type,omitempty" jsonschema:"text (default) or html"`
	Mentions    []OutboundMention `json:"mentions,omitempty" jsonschema:"optional mentions; content must include matching <at id=\"N\"> tags when provided"`
}

func sendChannelMessage(client *graph.Client) mcp.ToolHandlerFor[SendChannelMessageInput, SentMessage] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in SendChannelMessageInput) (*mcp.CallToolResult, SentMessage, error) {
		contentType, err := validateMessage(in.ContentType, in.Content)
		if err != nil {
			return nil, SentMessage{}, err
		}
		mentions, err := buildMentions(contentType, in.Mentions)
		if err != nil {
			return nil, SentMessage{}, err
		}
		message, err := client.SendChannelMessage(ctx, in.TeamID, in.ChannelID, contentType, in.Content, mentions)
		if err != nil {
			return nil, SentMessage{}, fmt.Errorf("send message to channel %s: %w", in.ChannelID, err)
		}
		return nil, sentMessage(message), nil
	}
}

// ReplyToChannelMessageInput is the input for teams_reply_to_channel_message.
type ReplyToChannelMessageInput struct {
	TeamID      string            `json:"team_id" jsonschema:"the team id"`
	ChannelID   string            `json:"channel_id" jsonschema:"the channel id"`
	MessageID   string            `json:"message_id" jsonschema:"the root channel message id to reply to"`
	Content     string            `json:"content" jsonschema:"reply body to send"`
	ContentType string            `json:"content_type,omitempty" jsonschema:"text (default) or html"`
	Mentions    []OutboundMention `json:"mentions,omitempty" jsonschema:"optional mentions; content must include matching <at id=\"N\"> tags when provided"`
}

func replyToChannelMessage(client *graph.Client) mcp.ToolHandlerFor[ReplyToChannelMessageInput, SentMessage] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ReplyToChannelMessageInput) (*mcp.CallToolResult, SentMessage, error) {
		contentType, err := validateMessage(in.ContentType, in.Content)
		if err != nil {
			return nil, SentMessage{}, err
		}
		mentions, err := buildMentions(contentType, in.Mentions)
		if err != nil {
			return nil, SentMessage{}, err
		}
		message, err := client.ReplyToChannelMessage(ctx, in.TeamID, in.ChannelID, in.MessageID, contentType, in.Content, mentions)
		if err != nil {
			return nil, SentMessage{}, fmt.Errorf("reply to channel message %s: %w", in.MessageID, err)
		}
		return nil, sentMessage(message), nil
	}
}

// SendChatMessageInput is the input for teams_send_chat_message.
type SendChatMessageInput struct {
	ChatID      string            `json:"chat_id" jsonschema:"the id of an existing chat"`
	Content     string            `json:"content" jsonschema:"message body to send"`
	ContentType string            `json:"content_type,omitempty" jsonschema:"text (default) or html"`
	Mentions    []OutboundMention `json:"mentions,omitempty" jsonschema:"optional mentions; content must include matching <at id=\"N\"> tags when provided"`
}

func sendChatMessage(client *graph.Client) mcp.ToolHandlerFor[SendChatMessageInput, SentMessage] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in SendChatMessageInput) (*mcp.CallToolResult, SentMessage, error) {
		contentType, err := validateMessage(in.ContentType, in.Content)
		if err != nil {
			return nil, SentMessage{}, err
		}
		mentions, err := buildMentions(contentType, in.Mentions)
		if err != nil {
			return nil, SentMessage{}, err
		}
		message, err := client.SendChatMessage(ctx, in.ChatID, contentType, in.Content, mentions)
		if err != nil {
			return nil, SentMessage{}, fmt.Errorf("send message to chat %s: %w", in.ChatID, err)
		}
		return nil, sentMessage(message), nil
	}
}

// UpdateChannelMessageInput is the input for teams_update_channel_message.
type UpdateChannelMessageInput struct {
	TeamID      string            `json:"team_id" jsonschema:"the team id"`
	ChannelID   string            `json:"channel_id" jsonschema:"the channel id"`
	MessageID   string            `json:"message_id" jsonschema:"the message id to edit"`
	Content     string            `json:"content" jsonschema:"replacement message body"`
	ContentType string            `json:"content_type,omitempty" jsonschema:"text (default) or html"`
	Mentions    []OutboundMention `json:"mentions,omitempty" jsonschema:"optional mentions; content must include matching <at id=\"N\"> tags when provided"`
}

func updateChannelMessage(client *graph.Client) mcp.ToolHandlerFor[UpdateChannelMessageInput, SentMessage] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in UpdateChannelMessageInput) (*mcp.CallToolResult, SentMessage, error) {
		contentType, err := validateMessage(in.ContentType, in.Content)
		if err != nil {
			return nil, SentMessage{}, err
		}
		mentions, err := buildMentions(contentType, in.Mentions)
		if err != nil {
			return nil, SentMessage{}, err
		}
		message, err := client.UpdateChannelMessage(ctx, in.TeamID, in.ChannelID, in.MessageID, contentType, in.Content, mentions)
		if err != nil {
			return nil, SentMessage{}, fmt.Errorf("update channel message %s: %w", in.MessageID, err)
		}
		return nil, sentMessage(message), nil
	}
}

// UpdateChatMessageInput is the input for teams_update_chat_message.
type UpdateChatMessageInput struct {
	ChatID      string            `json:"chat_id" jsonschema:"the chat id"`
	MessageID   string            `json:"message_id" jsonschema:"the message id to edit"`
	Content     string            `json:"content" jsonschema:"replacement message body"`
	ContentType string            `json:"content_type,omitempty" jsonschema:"text (default) or html"`
	Mentions    []OutboundMention `json:"mentions,omitempty" jsonschema:"optional mentions; content must include matching <at id=\"N\"> tags when provided"`
}

func updateChatMessage(client *graph.Client) mcp.ToolHandlerFor[UpdateChatMessageInput, SentMessage] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in UpdateChatMessageInput) (*mcp.CallToolResult, SentMessage, error) {
		contentType, err := validateMessage(in.ContentType, in.Content)
		if err != nil {
			return nil, SentMessage{}, err
		}
		mentions, err := buildMentions(contentType, in.Mentions)
		if err != nil {
			return nil, SentMessage{}, err
		}
		message, err := client.UpdateChatMessage(ctx, in.ChatID, in.MessageID, contentType, in.Content, mentions)
		if err != nil {
			return nil, SentMessage{}, fmt.Errorf("update chat message %s: %w", in.MessageID, err)
		}
		return nil, sentMessage(message), nil
	}
}

// DeleteChannelMessageInput is the input for teams_delete_channel_message.
type DeleteChannelMessageInput struct {
	TeamID    string `json:"team_id" jsonschema:"the team id"`
	ChannelID string `json:"channel_id" jsonschema:"the channel id"`
	MessageID string `json:"message_id" jsonschema:"the message id to delete"`
}

func deleteChannelMessage(client *graph.Client) mcp.ToolHandlerFor[DeleteChannelMessageInput, DeletedMessage] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in DeleteChannelMessageInput) (*mcp.CallToolResult, DeletedMessage, error) {
		if err := client.DeleteChannelMessage(ctx, in.TeamID, in.ChannelID, in.MessageID); err != nil {
			return nil, DeletedMessage{}, fmt.Errorf("delete channel message %s: %w", in.MessageID, err)
		}
		return nil, DeletedMessage{MessageID: in.MessageID}, nil
	}
}

// DeleteChatMessageInput is the input for teams_delete_chat_message.
type DeleteChatMessageInput struct {
	ChatID    string `json:"chat_id" jsonschema:"the chat id"`
	MessageID string `json:"message_id" jsonschema:"the message id to delete"`
}

func deleteChatMessage(client *graph.Client) mcp.ToolHandlerFor[DeleteChatMessageInput, DeletedMessage] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in DeleteChatMessageInput) (*mcp.CallToolResult, DeletedMessage, error) {
		if err := client.DeleteChatMessage(ctx, in.ChatID, in.MessageID); err != nil {
			return nil, DeletedMessage{}, fmt.Errorf("delete chat message %s: %w", in.MessageID, err)
		}
		return nil, DeletedMessage{MessageID: in.MessageID}, nil
	}
}

func registerWriteTools(server *mcp.Server, client *graph.Client) {
	mcp.AddTool(server, &mcp.Tool{Name: "teams_create_chat", Description: "Create a new one-on-one or group Teams chat.", Annotations: nonDestructiveHint}, createChat(client))
	mcp.AddTool(server, &mcp.Tool{Name: "teams_send_channel_message", Description: "Send a new root message to a Teams channel.", Annotations: nonDestructiveHint}, sendChannelMessage(client))
	mcp.AddTool(server, &mcp.Tool{Name: "teams_update_channel_message", Description: "Edit an existing root message in a Teams channel.", Annotations: nonDestructiveHint}, updateChannelMessage(client))
	mcp.AddTool(server, &mcp.Tool{Name: "teams_delete_channel_message", Description: "Soft-delete an existing root message in a Teams channel.", Annotations: destructiveHint}, deleteChannelMessage(client))
	mcp.AddTool(server, &mcp.Tool{Name: "teams_reply_to_channel_message", Description: "Reply to a root message in a Teams channel.", Annotations: nonDestructiveHint}, replyToChannelMessage(client))
	mcp.AddTool(server, &mcp.Tool{Name: "teams_send_chat_message", Description: "Send a message to an existing one-on-one, group, or meeting chat.", Annotations: nonDestructiveHint}, sendChatMessage(client))
	mcp.AddTool(server, &mcp.Tool{Name: "teams_update_chat_message", Description: "Edit an existing message in a Teams chat.", Annotations: nonDestructiveHint}, updateChatMessage(client))
	mcp.AddTool(server, &mcp.Tool{Name: "teams_delete_chat_message", Description: "Soft-delete an existing message in a Teams chat.", Annotations: destructiveHint}, deleteChatMessage(client))
}
