// Package mcpserver builds the teams-mcp MCP server. Read tools are always
// registered; message-sending tools require readwrite mode.
package mcpserver

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"teams-mcp/internal/graph"
)

var readOnlyHint = &mcp.ToolAnnotations{ReadOnlyHint: true}

// UserSummary describes the user whose Teams data the server accesses.
type UserSummary struct {
	ID                string `json:"id" jsonschema:"the Microsoft Graph user id"`
	DisplayName       string `json:"display_name,omitempty" jsonschema:"the user's display name"`
	UserPrincipalName string `json:"user_principal_name,omitempty" jsonschema:"the user's sign-in name"`
	Mail              string `json:"mail,omitempty" jsonschema:"the user's email address"`
}

func getCurrentUser(client *graph.Client) mcp.ToolHandlerFor[struct{}, UserSummary] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, UserSummary, error) {
		user, err := client.GetUser(ctx)
		if err != nil {
			return nil, UserSummary{}, fmt.Errorf("get current Teams user: %w", err)
		}
		return nil, UserSummary{
			ID: user.ID, DisplayName: user.DisplayName,
			UserPrincipalName: user.UserPrincipalName, Mail: user.Mail,
		}, nil
	}
}

// TeamSummary is a compact view of a Microsoft Team.
type TeamSummary struct {
	ID          string `json:"id" jsonschema:"the team id"`
	DisplayName string `json:"display_name,omitempty" jsonschema:"the team display name"`
	Description string `json:"description,omitempty" jsonschema:"the team description"`
	IsArchived  bool   `json:"is_archived,omitempty" jsonschema:"whether the team is archived"`
	TenantID    string `json:"tenant_id,omitempty" jsonschema:"the owning Microsoft Entra tenant id"`
}

// ListJoinedTeamsOutput is the output of teams_list_joined_teams.
type ListJoinedTeamsOutput struct {
	Teams []TeamSummary `json:"teams" jsonschema:"teams the target user directly belongs to"`
}

func listJoinedTeams(client *graph.Client) mcp.ToolHandlerFor[struct{}, ListJoinedTeamsOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, ListJoinedTeamsOutput, error) {
		page, err := client.ListJoinedTeams(ctx)
		if err != nil {
			return nil, ListJoinedTeamsOutput{}, fmt.Errorf("list joined teams: %w", err)
		}
		out := ListJoinedTeamsOutput{Teams: make([]TeamSummary, len(page.Value))}
		for i, team := range page.Value {
			out.Teams[i] = TeamSummary{
				ID: team.ID, DisplayName: team.DisplayName, Description: team.Description,
				IsArchived: team.IsArchived, TenantID: team.TenantID,
			}
		}
		return nil, out, nil
	}
}

// ChannelSummary is a compact view of a Teams channel.
type ChannelSummary struct {
	ID             string `json:"id" jsonschema:"the channel id"`
	DisplayName    string `json:"display_name,omitempty" jsonschema:"the channel display name"`
	Description    string `json:"description,omitempty" jsonschema:"the channel description"`
	MembershipType string `json:"membership_type,omitempty" jsonschema:"standard, private, shared, or unknownFutureValue"`
	WebURL         string `json:"web_url,omitempty" jsonschema:"the channel URL in Microsoft Teams"`
}

// ListChannelsInput is the input for teams_list_channels.
type ListChannelsInput struct {
	TeamID   string `json:"team_id" jsonschema:"the team id returned by teams_list_joined_teams"`
	NextLink string `json:"next_link,omitempty" jsonschema:"opaque next_link returned by the previous call; omit for the first page"`
}

// ListChannelsOutput is one page of team channels.
type ListChannelsOutput struct {
	Channels []ChannelSummary `json:"channels" jsonschema:"channels in this page"`
	NextLink string           `json:"next_link,omitempty" jsonschema:"opaque URL to pass to the next call"`
}

func listChannels(client *graph.Client) mcp.ToolHandlerFor[ListChannelsInput, ListChannelsOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListChannelsInput) (*mcp.CallToolResult, ListChannelsOutput, error) {
		page, err := client.ListChannels(ctx, in.TeamID, in.NextLink)
		if err != nil {
			return nil, ListChannelsOutput{}, fmt.Errorf("list channels for team %s: %w", in.TeamID, err)
		}
		out := ListChannelsOutput{Channels: make([]ChannelSummary, len(page.Value)), NextLink: page.NextLink}
		for i, channel := range page.Value {
			out.Channels[i] = ChannelSummary{
				ID: channel.ID, DisplayName: channel.DisplayName, Description: channel.Description,
				MembershipType: channel.MembershipType, WebURL: channel.WebURL,
			}
		}
		return nil, out, nil
	}
}

// ChatSummary is a compact view of a Teams chat.
type ChatSummary struct {
	ID                  string `json:"id" jsonschema:"the chat id"`
	Topic               string `json:"topic,omitempty" jsonschema:"the chat topic, if present"`
	ChatType            string `json:"chat_type,omitempty" jsonschema:"oneOnOne, group, or meeting"`
	CreatedDateTime     string `json:"created_date_time,omitempty" jsonschema:"creation timestamp"`
	LastUpdatedDateTime string `json:"last_updated_date_time,omitempty" jsonschema:"last update timestamp"`
	WebURL              string `json:"web_url,omitempty" jsonschema:"the chat URL in Microsoft Teams"`
}

// ListChatsInput is the input for teams_list_chats.
type ListChatsInput struct {
	NextLink string `json:"next_link,omitempty" jsonschema:"opaque next_link returned by the previous call"`
	Top      int    `json:"top,omitempty" jsonschema:"page size from 1 to 50; defaults to 20"`
}

// ListChatsOutput is one page of chats.
type ListChatsOutput struct {
	Chats    []ChatSummary `json:"chats" jsonschema:"chats in this page"`
	NextLink string        `json:"next_link,omitempty" jsonschema:"opaque URL to pass to the next call"`
}

func listChats(client *graph.Client) mcp.ToolHandlerFor[ListChatsInput, ListChatsOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListChatsInput) (*mcp.CallToolResult, ListChatsOutput, error) {
		page, err := client.ListChats(ctx, in.NextLink, pageSize(in.Top))
		if err != nil {
			return nil, ListChatsOutput{}, fmt.Errorf("list chats: %w", err)
		}
		out := ListChatsOutput{Chats: make([]ChatSummary, len(page.Value)), NextLink: page.NextLink}
		for i, chat := range page.Value {
			out.Chats[i] = ChatSummary{
				ID: chat.ID, Topic: chat.Topic, ChatType: chat.ChatType,
				CreatedDateTime: chat.CreatedDateTime, LastUpdatedDateTime: chat.LastUpdatedDateTime, WebURL: chat.WebURL,
			}
		}
		return nil, out, nil
	}
}

// AttachmentSummary is attachment metadata attached to a message.
type AttachmentSummary struct {
	ID           string `json:"id,omitempty" jsonschema:"the attachment id"`
	Name         string `json:"name,omitempty" jsonschema:"the attachment name"`
	ContentType  string `json:"content_type,omitempty" jsonschema:"the attachment MIME or Teams content type"`
	ContentURL   string `json:"content_url,omitempty" jsonschema:"URL for attachment content, when provided"`
	ThumbnailURL string `json:"thumbnail_url,omitempty" jsonschema:"URL for an attachment thumbnail, when provided"`
}

// MessageSummary is a flattened Teams chat message.
type MessageSummary struct {
	ID                   string              `json:"id" jsonschema:"the message id"`
	ReplyToID            string              `json:"reply_to_id,omitempty" jsonschema:"the root message id when this is a reply"`
	SenderID             string              `json:"sender_id,omitempty" jsonschema:"the sender's user, application, or device id"`
	SenderName           string              `json:"sender_name,omitempty" jsonschema:"the sender's display name"`
	SenderType           string              `json:"sender_type,omitempty" jsonschema:"user, application, or device"`
	MessageType          string              `json:"message_type,omitempty" jsonschema:"message or system event type"`
	CreatedDateTime      string              `json:"created_date_time,omitempty" jsonschema:"creation timestamp"`
	LastModifiedDateTime string              `json:"last_modified_date_time,omitempty" jsonschema:"last modification timestamp"`
	DeletedDateTime      string              `json:"deleted_date_time,omitempty" jsonschema:"deletion timestamp, if deleted"`
	Subject              string              `json:"subject,omitempty" jsonschema:"message subject, if present"`
	Summary              string              `json:"summary,omitempty" jsonschema:"message summary, if present"`
	ContentType          string              `json:"content_type,omitempty" jsonschema:"text or html"`
	Content              string              `json:"content,omitempty" jsonschema:"the message body"`
	WebURL               string              `json:"web_url,omitempty" jsonschema:"the message URL in Microsoft Teams"`
	Attachments          []AttachmentSummary `json:"attachments,omitempty" jsonschema:"attachment metadata"`
}

func messageToSummary(message graph.ChatMessage) MessageSummary {
	out := MessageSummary{
		ID: message.ID, ReplyToID: message.ReplyToID, MessageType: message.MessageType,
		CreatedDateTime: message.CreatedDateTime, LastModifiedDateTime: message.LastModifiedDateTime,
		DeletedDateTime: message.DeletedDateTime, Subject: message.Subject, Summary: message.Summary,
		ContentType: message.Body.ContentType, Content: message.Body.Content, WebURL: message.WebURL,
	}
	if message.From != nil {
		switch {
		case message.From.User != nil:
			out.SenderType, out.SenderID, out.SenderName = "user", message.From.User.ID, message.From.User.DisplayName
		case message.From.Application != nil:
			out.SenderType, out.SenderID, out.SenderName = "application", message.From.Application.ID, message.From.Application.DisplayName
		case message.From.Device != nil:
			out.SenderType, out.SenderID, out.SenderName = "device", message.From.Device.ID, message.From.Device.DisplayName
		}
	}
	if len(message.Attachments) > 0 {
		out.Attachments = make([]AttachmentSummary, len(message.Attachments))
		for i, attachment := range message.Attachments {
			out.Attachments[i] = AttachmentSummary{
				ID: attachment.ID, Name: attachment.Name, ContentType: attachment.ContentType,
				ContentURL: attachment.ContentURL, ThumbnailURL: attachment.ThumbnailURL,
			}
		}
	}
	return out
}

func pageSize(requested int) int {
	if requested <= 0 {
		return 20
	}
	if requested > 50 {
		return 50
	}
	return requested
}

// ListChannelMessagesInput is the input for teams_list_channel_messages.
type ListChannelMessagesInput struct {
	TeamID    string `json:"team_id" jsonschema:"the team id"`
	ChannelID string `json:"channel_id" jsonschema:"the channel id"`
	NextLink  string `json:"next_link,omitempty" jsonschema:"opaque next_link returned by the previous call"`
	Top       int    `json:"top,omitempty" jsonschema:"page size from 1 to 50; defaults to 20"`
}

// ListChannelMessagesOutput is one page of root channel messages.
type ListChannelMessagesOutput struct {
	Messages []MessageSummary `json:"messages" jsonschema:"root channel messages in this page"`
	NextLink string           `json:"next_link,omitempty" jsonschema:"opaque URL to pass to the next call"`
}

func listChannelMessages(client *graph.Client) mcp.ToolHandlerFor[ListChannelMessagesInput, ListChannelMessagesOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListChannelMessagesInput) (*mcp.CallToolResult, ListChannelMessagesOutput, error) {
		page, err := client.ListChannelMessages(ctx, in.TeamID, in.ChannelID, in.NextLink, pageSize(in.Top))
		if err != nil {
			return nil, ListChannelMessagesOutput{}, fmt.Errorf("list messages in channel %s: %w", in.ChannelID, err)
		}
		out := ListChannelMessagesOutput{Messages: make([]MessageSummary, len(page.Value)), NextLink: page.NextLink}
		for i, message := range page.Value {
			out.Messages[i] = messageToSummary(message)
		}
		return nil, out, nil
	}
}

// ListChannelMessageRepliesInput is the input for teams_list_channel_message_replies.
type ListChannelMessageRepliesInput struct {
	TeamID    string `json:"team_id" jsonschema:"the team id"`
	ChannelID string `json:"channel_id" jsonschema:"the channel id"`
	MessageID string `json:"message_id" jsonschema:"the root channel message id"`
	NextLink  string `json:"next_link,omitempty" jsonschema:"opaque next_link returned by the previous call"`
	Top       int    `json:"top,omitempty" jsonschema:"page size from 1 to 50; defaults to 20"`
}

// ListChannelMessageRepliesOutput is one page of channel replies.
type ListChannelMessageRepliesOutput struct {
	Replies  []MessageSummary `json:"replies" jsonschema:"message replies in this page"`
	NextLink string           `json:"next_link,omitempty" jsonschema:"opaque URL to pass to the next call"`
}

func listChannelMessageReplies(client *graph.Client) mcp.ToolHandlerFor[ListChannelMessageRepliesInput, ListChannelMessageRepliesOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListChannelMessageRepliesInput) (*mcp.CallToolResult, ListChannelMessageRepliesOutput, error) {
		page, err := client.ListChannelMessageReplies(ctx, in.TeamID, in.ChannelID, in.MessageID, in.NextLink, pageSize(in.Top))
		if err != nil {
			return nil, ListChannelMessageRepliesOutput{}, fmt.Errorf("list replies to channel message %s: %w", in.MessageID, err)
		}
		out := ListChannelMessageRepliesOutput{Replies: make([]MessageSummary, len(page.Value)), NextLink: page.NextLink}
		for i, message := range page.Value {
			out.Replies[i] = messageToSummary(message)
		}
		return nil, out, nil
	}
}

// ListChatMessagesInput is the input for teams_list_chat_messages.
type ListChatMessagesInput struct {
	ChatID   string `json:"chat_id" jsonschema:"the chat id returned by teams_list_chats"`
	NextLink string `json:"next_link,omitempty" jsonschema:"opaque next_link returned by the previous call"`
	Top      int    `json:"top,omitempty" jsonschema:"page size from 1 to 50; defaults to 20"`
}

// ListChatMessagesOutput is one page of chat messages.
type ListChatMessagesOutput struct {
	Messages []MessageSummary `json:"messages" jsonschema:"chat messages in this page"`
	NextLink string           `json:"next_link,omitempty" jsonschema:"opaque URL to pass to the next call"`
}

func listChatMessages(client *graph.Client) mcp.ToolHandlerFor[ListChatMessagesInput, ListChatMessagesOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListChatMessagesInput) (*mcp.CallToolResult, ListChatMessagesOutput, error) {
		page, err := client.ListChatMessages(ctx, in.ChatID, in.NextLink, pageSize(in.Top))
		if err != nil {
			return nil, ListChatMessagesOutput{}, fmt.Errorf("list messages in chat %s: %w", in.ChatID, err)
		}
		out := ListChatMessagesOutput{Messages: make([]MessageSummary, len(page.Value)), NextLink: page.NextLink}
		for i, message := range page.Value {
			out.Messages[i] = messageToSummary(message)
		}
		return nil, out, nil
	}
}

func registerReadTools(server *mcp.Server, client *graph.Client) {
	mcp.AddTool(server, &mcp.Tool{Name: "teams_get_current_user", Description: "Get the Microsoft 365 user whose Teams data this server accesses.", Annotations: readOnlyHint}, getCurrentUser(client))
	mcp.AddTool(server, &mcp.Tool{Name: "teams_list_joined_teams", Description: "List teams the target user directly belongs to.", Annotations: readOnlyHint}, listJoinedTeams(client))
	mcp.AddTool(server, &mcp.Tool{Name: "teams_list_channels", Description: "List channels in a Microsoft Team.", Annotations: readOnlyHint}, listChannels(client))
	mcp.AddTool(server, &mcp.Tool{Name: "teams_list_channel_messages", Description: "List root messages in a Teams channel. Use teams_list_channel_message_replies for replies.", Annotations: readOnlyHint}, listChannelMessages(client))
	mcp.AddTool(server, &mcp.Tool{Name: "teams_list_channel_message_replies", Description: "List replies to a root Teams channel message.", Annotations: readOnlyHint}, listChannelMessageReplies(client))
	mcp.AddTool(server, &mcp.Tool{Name: "teams_list_chats", Description: "List the target user's one-on-one, group, and meeting chats.", Annotations: readOnlyHint}, listChats(client))
	mcp.AddTool(server, &mcp.Tool{Name: "teams_list_chat_messages", Description: "List messages in a Teams chat.", Annotations: readOnlyHint}, listChatMessages(client))
}
