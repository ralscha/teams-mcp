package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"teams-mcp/internal/graph"
)

var nonDestructiveHint = &mcp.ToolAnnotations{DestructiveHint: new(false)}

// SentMessage identifies a message created by a write tool.
type SentMessage struct {
	MessageID       string `json:"message_id" jsonschema:"the created message id"`
	CreatedDateTime string `json:"created_date_time,omitempty" jsonschema:"message creation timestamp"`
	WebURL          string `json:"web_url,omitempty" jsonschema:"the message URL in Microsoft Teams"`
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

// SendChannelMessageInput is the input for teams_send_channel_message.
type SendChannelMessageInput struct {
	TeamID      string `json:"team_id" jsonschema:"the team id"`
	ChannelID   string `json:"channel_id" jsonschema:"the channel id"`
	Content     string `json:"content" jsonschema:"message body to send"`
	ContentType string `json:"content_type,omitempty" jsonschema:"text (default) or html"`
}

func sendChannelMessage(client *graph.Client) mcp.ToolHandlerFor[SendChannelMessageInput, SentMessage] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in SendChannelMessageInput) (*mcp.CallToolResult, SentMessage, error) {
		contentType, err := validateMessage(in.ContentType, in.Content)
		if err != nil {
			return nil, SentMessage{}, err
		}
		message, err := client.SendChannelMessage(ctx, in.TeamID, in.ChannelID, contentType, in.Content)
		if err != nil {
			return nil, SentMessage{}, fmt.Errorf("send message to channel %s: %w", in.ChannelID, err)
		}
		return nil, sentMessage(message), nil
	}
}

// ReplyToChannelMessageInput is the input for teams_reply_to_channel_message.
type ReplyToChannelMessageInput struct {
	TeamID      string `json:"team_id" jsonschema:"the team id"`
	ChannelID   string `json:"channel_id" jsonschema:"the channel id"`
	MessageID   string `json:"message_id" jsonschema:"the root channel message id to reply to"`
	Content     string `json:"content" jsonschema:"reply body to send"`
	ContentType string `json:"content_type,omitempty" jsonschema:"text (default) or html"`
}

func replyToChannelMessage(client *graph.Client) mcp.ToolHandlerFor[ReplyToChannelMessageInput, SentMessage] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ReplyToChannelMessageInput) (*mcp.CallToolResult, SentMessage, error) {
		contentType, err := validateMessage(in.ContentType, in.Content)
		if err != nil {
			return nil, SentMessage{}, err
		}
		message, err := client.ReplyToChannelMessage(ctx, in.TeamID, in.ChannelID, in.MessageID, contentType, in.Content)
		if err != nil {
			return nil, SentMessage{}, fmt.Errorf("reply to channel message %s: %w", in.MessageID, err)
		}
		return nil, sentMessage(message), nil
	}
}

// SendChatMessageInput is the input for teams_send_chat_message.
type SendChatMessageInput struct {
	ChatID      string `json:"chat_id" jsonschema:"the id of an existing chat"`
	Content     string `json:"content" jsonschema:"message body to send"`
	ContentType string `json:"content_type,omitempty" jsonschema:"text (default) or html"`
}

func sendChatMessage(client *graph.Client) mcp.ToolHandlerFor[SendChatMessageInput, SentMessage] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in SendChatMessageInput) (*mcp.CallToolResult, SentMessage, error) {
		contentType, err := validateMessage(in.ContentType, in.Content)
		if err != nil {
			return nil, SentMessage{}, err
		}
		message, err := client.SendChatMessage(ctx, in.ChatID, contentType, in.Content)
		if err != nil {
			return nil, SentMessage{}, fmt.Errorf("send message to chat %s: %w", in.ChatID, err)
		}
		return nil, sentMessage(message), nil
	}
}

func registerWriteTools(server *mcp.Server, client *graph.Client) {
	mcp.AddTool(server, &mcp.Tool{Name: "teams_send_channel_message", Description: "Send a new root message to a Teams channel.", Annotations: nonDestructiveHint}, sendChannelMessage(client))
	mcp.AddTool(server, &mcp.Tool{Name: "teams_reply_to_channel_message", Description: "Reply to a root message in a Teams channel.", Annotations: nonDestructiveHint}, replyToChannelMessage(client))
	mcp.AddTool(server, &mcp.Tool{Name: "teams_send_chat_message", Description: "Send a message to an existing one-on-one, group, or meeting chat.", Annotations: nonDestructiveHint}, sendChatMessage(client))
}
