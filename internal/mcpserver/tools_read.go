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

// GetUserInput is the input for teams_get_user.
type GetUserInput struct {
	UserID string `json:"user_id" jsonschema:"the user object id or user principal name (UPN)"`
}

func getUser(client *graph.Client) mcp.ToolHandlerFor[GetUserInput, UserSummary] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in GetUserInput) (*mcp.CallToolResult, UserSummary, error) {
		user, err := client.GetUserByID(ctx, in.UserID)
		if err != nil {
			return nil, UserSummary{}, fmt.Errorf("get user %s: %w", in.UserID, err)
		}
		return nil, UserSummary{
			ID: user.ID, DisplayName: user.DisplayName,
			UserPrincipalName: user.UserPrincipalName, Mail: user.Mail,
		}, nil
	}
}

// ListUsersInput is the input for teams_list_users.
type ListUsersInput struct {
	Query    string `json:"query,omitempty" jsonschema:"optional prefix filter for display name, mail, or UPN"`
	NextLink string `json:"next_link,omitempty" jsonschema:"opaque next_link returned by the previous call"`
	Top      int    `json:"top,omitempty" jsonschema:"page size from 1 to 50; defaults to 20"`
}

// ListUsersOutput is one page of users.
type ListUsersOutput struct {
	Users    []UserSummary `json:"users" jsonschema:"users in this page"`
	NextLink string        `json:"next_link,omitempty" jsonschema:"opaque URL to pass to the next call"`
}

func listUsers(client *graph.Client) mcp.ToolHandlerFor[ListUsersInput, ListUsersOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListUsersInput) (*mcp.CallToolResult, ListUsersOutput, error) {
		page, err := client.ListUsers(ctx, in.Query, in.NextLink, pageSize(in.Top))
		if err != nil {
			return nil, ListUsersOutput{}, fmt.Errorf("list users: %w", err)
		}
		out := ListUsersOutput{Users: make([]UserSummary, len(page.Value)), NextLink: page.NextLink}
		for i, user := range page.Value {
			out.Users[i] = UserSummary{
				ID: user.ID, DisplayName: user.DisplayName,
				UserPrincipalName: user.UserPrincipalName, Mail: user.Mail,
			}
		}
		return nil, out, nil
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
	Teams    []TeamSummary `json:"teams" jsonschema:"teams in this page that the target user directly belongs to"`
	NextLink string        `json:"next_link,omitempty" jsonschema:"opaque URL to pass to the next call"`
}

// ListJoinedTeamsInput is the input for teams_list_joined_teams.
type ListJoinedTeamsInput struct {
	NextLink string `json:"next_link,omitempty" jsonschema:"opaque next_link returned by the previous call"`
}

func listJoinedTeams(client *graph.Client) mcp.ToolHandlerFor[ListJoinedTeamsInput, ListJoinedTeamsOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListJoinedTeamsInput) (*mcp.CallToolResult, ListJoinedTeamsOutput, error) {
		page, err := client.ListJoinedTeams(ctx, in.NextLink)
		if err != nil {
			return nil, ListJoinedTeamsOutput{}, fmt.Errorf("list joined teams: %w", err)
		}
		out := ListJoinedTeamsOutput{Teams: make([]TeamSummary, len(page.Value)), NextLink: page.NextLink}
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

// GetChannelInput is the input for teams_get_channel.
type GetChannelInput struct {
	TeamID    string `json:"team_id" jsonschema:"the team id returned by teams_list_joined_teams"`
	ChannelID string `json:"channel_id" jsonschema:"the channel id"`
}

func getChannel(client *graph.Client) mcp.ToolHandlerFor[GetChannelInput, ChannelSummary] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in GetChannelInput) (*mcp.CallToolResult, ChannelSummary, error) {
		channel, err := client.GetChannel(ctx, in.TeamID, in.ChannelID)
		if err != nil {
			return nil, ChannelSummary{}, fmt.Errorf("get channel %s: %w", in.ChannelID, err)
		}
		return nil, ChannelSummary{
			ID: channel.ID, DisplayName: channel.DisplayName, Description: channel.Description,
			MembershipType: channel.MembershipType, WebURL: channel.WebURL,
		}, nil
	}
}

// TeamMemberSummary is a compact view of a team member.
type TeamMemberSummary struct {
	ID          string   `json:"id,omitempty" jsonschema:"the member id"`
	UserID      string   `json:"user_id,omitempty" jsonschema:"the Entra user id when the member is an AAD user"`
	DisplayName string   `json:"display_name,omitempty" jsonschema:"the member display name"`
	Email       string   `json:"email,omitempty" jsonschema:"the member email address, when provided by Graph"`
	Roles       []string `json:"roles,omitempty" jsonschema:"member roles, such as owner"`
	TenantID    string   `json:"tenant_id,omitempty" jsonschema:"the member tenant id"`
	Type        string   `json:"type,omitempty" jsonschema:"the Graph @odata.type of the conversation member"`
}

func teamMemberSummary(member graph.ConversationMember) TeamMemberSummary {
	return TeamMemberSummary{
		ID: member.ID, UserID: member.UserID, DisplayName: member.DisplayName,
		Email: member.Email, Roles: member.Roles, TenantID: member.TenantID, Type: member.ODataType,
	}
}

// ListTeamMembersInput is the input for teams_list_team_members.
type ListTeamMembersInput struct {
	TeamID   string `json:"team_id" jsonschema:"the team id returned by teams_list_joined_teams"`
	NextLink string `json:"next_link,omitempty" jsonschema:"opaque next_link returned by the previous call"`
	Top      int    `json:"top,omitempty" jsonschema:"page size from 1 to 50; defaults to 20"`
}

// ListTeamMembersOutput is one page of team members.
type ListTeamMembersOutput struct {
	Members  []TeamMemberSummary `json:"members" jsonschema:"members in this team page"`
	NextLink string              `json:"next_link,omitempty" jsonschema:"opaque URL to pass to the next call"`
}

func listTeamMembers(client *graph.Client) mcp.ToolHandlerFor[ListTeamMembersInput, ListTeamMembersOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListTeamMembersInput) (*mcp.CallToolResult, ListTeamMembersOutput, error) {
		page, err := client.ListTeamMembers(ctx, in.TeamID, in.NextLink, pageSize(in.Top))
		if err != nil {
			return nil, ListTeamMembersOutput{}, fmt.Errorf("list members in team %s: %w", in.TeamID, err)
		}
		out := ListTeamMembersOutput{Members: make([]TeamMemberSummary, len(page.Value)), NextLink: page.NextLink}
		for i, member := range page.Value {
			out.Members[i] = teamMemberSummary(member)
		}
		return nil, out, nil
	}
}

// ChatSummary is a compact view of a Teams chat.
type ChatSummary struct {
	ID                  string                   `json:"id" jsonschema:"the chat id"`
	Topic               string                   `json:"topic,omitempty" jsonschema:"the chat topic, if present"`
	ChatType            string                   `json:"chat_type,omitempty" jsonschema:"oneOnOne, group, or meeting"`
	CreatedDateTime     string                   `json:"created_date_time,omitempty" jsonschema:"creation timestamp"`
	LastUpdatedDateTime string                   `json:"last_updated_date_time,omitempty" jsonschema:"last update timestamp"`
	WebURL              string                   `json:"web_url,omitempty" jsonschema:"the chat URL in Microsoft Teams"`
	Participants        []ChatParticipantSummary `json:"participants,omitempty" jsonschema:"chat participants returned by Microsoft Graph when available"`
}

// ChatParticipantSummary is a compact view of a chat member.
type ChatParticipantSummary struct {
	ID          string   `json:"id,omitempty" jsonschema:"the member id"`
	UserID      string   `json:"user_id,omitempty" jsonschema:"the Entra user id when the participant is an AAD user"`
	DisplayName string   `json:"display_name,omitempty" jsonschema:"the participant display name"`
	Email       string   `json:"email,omitempty" jsonschema:"the participant email address, when provided by Graph"`
	Roles       []string `json:"roles,omitempty" jsonschema:"participant roles, such as owner"`
	TenantID    string   `json:"tenant_id,omitempty" jsonschema:"the participant tenant id"`
	Type        string   `json:"type,omitempty" jsonschema:"the Graph @odata.type of the conversation member"`
}

func chatParticipantSummary(member graph.ConversationMember) ChatParticipantSummary {
	return ChatParticipantSummary{
		ID: member.ID, UserID: member.UserID, DisplayName: member.DisplayName,
		Email: member.Email, Roles: member.Roles, TenantID: member.TenantID, Type: member.ODataType,
	}
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
			summary := ChatSummary{
				ID: chat.ID, Topic: chat.Topic, ChatType: chat.ChatType,
				CreatedDateTime: chat.CreatedDateTime, LastUpdatedDateTime: chat.LastUpdatedDateTime, WebURL: chat.WebURL,
			}
			if len(chat.Members) > 0 {
				summary.Participants = make([]ChatParticipantSummary, len(chat.Members))
				for j, member := range chat.Members {
					summary.Participants[j] = chatParticipantSummary(member)
				}
			}
			out.Chats[i] = summary
		}
		return nil, out, nil
	}
}

// GetChatInput is the input for teams_get_chat.
type GetChatInput struct {
	ChatID string `json:"chat_id" jsonschema:"the chat id returned by teams_list_chats"`
}

func getChat(client *graph.Client) mcp.ToolHandlerFor[GetChatInput, ChatSummary] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in GetChatInput) (*mcp.CallToolResult, ChatSummary, error) {
		chat, err := client.GetChat(ctx, in.ChatID)
		if err != nil {
			return nil, ChatSummary{}, fmt.Errorf("get chat %s: %w", in.ChatID, err)
		}
		summary := ChatSummary{
			ID: chat.ID, Topic: chat.Topic, ChatType: chat.ChatType,
			CreatedDateTime: chat.CreatedDateTime, LastUpdatedDateTime: chat.LastUpdatedDateTime, WebURL: chat.WebURL,
		}
		if len(chat.Members) > 0 {
			summary.Participants = make([]ChatParticipantSummary, len(chat.Members))
			for i, member := range chat.Members {
				summary.Participants[i] = chatParticipantSummary(member)
			}
		}
		return nil, summary, nil
	}
}

// ListChatMembersInput is the input for teams_list_chat_members.
type ListChatMembersInput struct {
	ChatID   string `json:"chat_id" jsonschema:"the chat id returned by teams_list_chats"`
	NextLink string `json:"next_link,omitempty" jsonschema:"opaque next_link returned by the previous call"`
	Top      int    `json:"top,omitempty" jsonschema:"page size from 1 to 50; defaults to 20"`
}

// ListChatMembersOutput is one page of chat members.
type ListChatMembersOutput struct {
	Members  []ChatParticipantSummary `json:"members" jsonschema:"members in this chat page"`
	NextLink string                   `json:"next_link,omitempty" jsonschema:"opaque URL to pass to the next call"`
}

func listChatMembers(client *graph.Client) mcp.ToolHandlerFor[ListChatMembersInput, ListChatMembersOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ListChatMembersInput) (*mcp.CallToolResult, ListChatMembersOutput, error) {
		page, err := client.ListChatMembers(ctx, in.ChatID, in.NextLink, pageSize(in.Top))
		if err != nil {
			return nil, ListChatMembersOutput{}, fmt.Errorf("list members in chat %s: %w", in.ChatID, err)
		}
		out := ListChatMembersOutput{Members: make([]ChatParticipantSummary, len(page.Value)), NextLink: page.NextLink}
		for i, member := range page.Value {
			out.Members[i] = chatParticipantSummary(member)
		}
		return nil, out, nil
	}
}

// SearchMessagesInput is the input for teams_search_messages.
type SearchMessagesInput struct {
	Query string `json:"query" jsonschema:"free-text search query over Teams messages"`
	From  int    `json:"from,omitempty" jsonschema:"result offset, defaults to 0"`
	Size  int    `json:"size,omitempty" jsonschema:"page size from 1 to 50; defaults to 20"`
}

// SearchMessagesOutput is one page of Teams message search results.
type SearchMessagesOutput struct {
	Messages             []MessageSummary `json:"messages" jsonschema:"matching Teams messages in this result page"`
	Total                int              `json:"total,omitempty" jsonschema:"total matches reported by Graph for this request"`
	MoreResultsAvailable bool             `json:"more_results_available,omitempty" jsonschema:"whether Graph reports more results after this page"`
}

func searchMessages(client *graph.Client) mcp.ToolHandlerFor[SearchMessagesInput, SearchMessagesOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in SearchMessagesInput) (*mcp.CallToolResult, SearchMessagesOutput, error) {
		if in.Query == "" {
			return nil, SearchMessagesOutput{}, fmt.Errorf("query must not be empty")
		}
		response, err := client.SearchMessages(ctx, in.Query, in.From, pageSize(in.Size))
		if err != nil {
			return nil, SearchMessagesOutput{}, fmt.Errorf("search messages: %w", err)
		}
		out := SearchMessagesOutput{}
		for _, result := range response.Value {
			for _, container := range result.HitsContainers {
				out.Total += container.Total
				out.MoreResultsAvailable = out.MoreResultsAvailable || container.MoreResultsAvailable
				for _, hit := range container.Hits {
					out.Messages = append(out.Messages, messageToSummary(hit.Resource))
				}
			}
		}
		if out.Messages == nil {
			out.Messages = []MessageSummary{}
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

// ChannelIdentitySummary identifies the team and channel for channel messages.
type ChannelIdentitySummary struct {
	TeamID    string `json:"team_id,omitempty" jsonschema:"the containing team id"`
	ChannelID string `json:"channel_id,omitempty" jsonschema:"the containing channel id"`
}

// MentionSummary describes one mention embedded in a message.
type MentionSummary struct {
	ID          int    `json:"id" jsonschema:"the mention id used by message HTML"`
	MentionText string `json:"mention_text,omitempty" jsonschema:"display text associated with the mention"`
	Type        string `json:"type,omitempty" jsonschema:"user, application, or device"`
	MentionedID string `json:"mentioned_id,omitempty" jsonschema:"the id of the mentioned identity"`
	DisplayName string `json:"display_name,omitempty" jsonschema:"display name of the mentioned identity"`
}

// MessageSummary is a flattened Teams chat message.
type MessageSummary struct {
	ID                   string                  `json:"id" jsonschema:"the message id"`
	ReplyToID            string                  `json:"reply_to_id,omitempty" jsonschema:"the root message id when this is a reply"`
	SenderID             string                  `json:"sender_id,omitempty" jsonschema:"the sender's user, application, or device id"`
	SenderName           string                  `json:"sender_name,omitempty" jsonschema:"the sender's display name"`
	SenderType           string                  `json:"sender_type,omitempty" jsonschema:"user, application, or device"`
	MessageType          string                  `json:"message_type,omitempty" jsonschema:"message or system event type"`
	CreatedDateTime      string                  `json:"created_date_time,omitempty" jsonschema:"creation timestamp"`
	LastModifiedDateTime string                  `json:"last_modified_date_time,omitempty" jsonschema:"last modification timestamp"`
	DeletedDateTime      string                  `json:"deleted_date_time,omitempty" jsonschema:"deletion timestamp, if deleted"`
	Subject              string                  `json:"subject,omitempty" jsonschema:"message subject, if present"`
	Summary              string                  `json:"summary,omitempty" jsonschema:"message summary, if present"`
	ContentType          string                  `json:"content_type,omitempty" jsonschema:"text or html"`
	Content              string                  `json:"content,omitempty" jsonschema:"the message body"`
	WebURL               string                  `json:"web_url,omitempty" jsonschema:"the message URL in Microsoft Teams"`
	ChannelIdentity      *ChannelIdentitySummary `json:"channel_identity,omitempty" jsonschema:"team and channel identifiers for channel messages"`
	Mentions             []MentionSummary        `json:"mentions,omitempty" jsonschema:"message mentions in display order"`
	Attachments          []AttachmentSummary     `json:"attachments,omitempty" jsonschema:"attachment metadata"`
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
	if message.ChannelIdentity != nil {
		out.ChannelIdentity = &ChannelIdentitySummary{TeamID: message.ChannelIdentity.TeamID, ChannelID: message.ChannelIdentity.ChannelID}
	}
	if len(message.Mentions) > 0 {
		out.Mentions = make([]MentionSummary, len(message.Mentions))
		for i, mention := range message.Mentions {
			summary := MentionSummary{ID: mention.ID, MentionText: mention.MentionText}
			switch {
			case mention.Mentioned.User != nil:
				summary.Type, summary.MentionedID, summary.DisplayName = "user", mention.Mentioned.User.ID, mention.Mentioned.User.DisplayName
			case mention.Mentioned.Application != nil:
				summary.Type, summary.MentionedID, summary.DisplayName = "application", mention.Mentioned.Application.ID, mention.Mentioned.Application.DisplayName
			case mention.Mentioned.Device != nil:
				summary.Type, summary.MentionedID, summary.DisplayName = "device", mention.Mentioned.Device.ID, mention.Mentioned.Device.DisplayName
			}
			out.Mentions[i] = summary
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

// GetChannelMessageInput is the input for teams_get_channel_message.
type GetChannelMessageInput struct {
	TeamID    string `json:"team_id" jsonschema:"the team id"`
	ChannelID string `json:"channel_id" jsonschema:"the channel id"`
	MessageID string `json:"message_id" jsonschema:"the channel message id"`
}

func getChannelMessage(client *graph.Client) mcp.ToolHandlerFor[GetChannelMessageInput, MessageSummary] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in GetChannelMessageInput) (*mcp.CallToolResult, MessageSummary, error) {
		message, err := client.GetChannelMessage(ctx, in.TeamID, in.ChannelID, in.MessageID)
		if err != nil {
			return nil, MessageSummary{}, fmt.Errorf("get channel message %s: %w", in.MessageID, err)
		}
		return nil, messageToSummary(*message), nil
	}
}

// GetChatMessageInput is the input for teams_get_chat_message.
type GetChatMessageInput struct {
	ChatID    string `json:"chat_id" jsonschema:"the chat id returned by teams_list_chats"`
	MessageID string `json:"message_id" jsonschema:"the chat message id"`
}

func getChatMessage(client *graph.Client) mcp.ToolHandlerFor[GetChatMessageInput, MessageSummary] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in GetChatMessageInput) (*mcp.CallToolResult, MessageSummary, error) {
		message, err := client.GetChatMessage(ctx, in.ChatID, in.MessageID)
		if err != nil {
			return nil, MessageSummary{}, fmt.Errorf("get chat message %s: %w", in.MessageID, err)
		}
		return nil, messageToSummary(*message), nil
	}
}

func registerReadTools(server *mcp.Server, client *graph.Client) {
	mcp.AddTool(server, &mcp.Tool{Name: "teams_get_current_user", Description: "Get the Microsoft 365 user whose Teams data this server accesses.", Annotations: readOnlyHint}, getCurrentUser(client))
	mcp.AddTool(server, &mcp.Tool{Name: "teams_get_user", Description: "Get one Microsoft 365 user by object id or UPN.", Annotations: readOnlyHint}, getUser(client))
	mcp.AddTool(server, &mcp.Tool{Name: "teams_list_users", Description: "List Microsoft 365 users with optional prefix filtering.", Annotations: readOnlyHint}, listUsers(client))
	mcp.AddTool(server, &mcp.Tool{Name: "teams_list_joined_teams", Description: "List teams the target user directly belongs to.", Annotations: readOnlyHint}, listJoinedTeams(client))
	mcp.AddTool(server, &mcp.Tool{Name: "teams_list_channels", Description: "List channels in a Microsoft Team.", Annotations: readOnlyHint}, listChannels(client))
	mcp.AddTool(server, &mcp.Tool{Name: "teams_get_channel", Description: "Get one channel in a Microsoft Team by id.", Annotations: readOnlyHint}, getChannel(client))
	mcp.AddTool(server, &mcp.Tool{Name: "teams_list_team_members", Description: "List members in a Microsoft Team.", Annotations: readOnlyHint}, listTeamMembers(client))
	mcp.AddTool(server, &mcp.Tool{Name: "teams_get_channel_message", Description: "Get a specific root message in a Teams channel by id.", Annotations: readOnlyHint}, getChannelMessage(client))
	mcp.AddTool(server, &mcp.Tool{Name: "teams_list_channel_messages", Description: "List root messages in a Teams channel. Use teams_list_channel_message_replies for replies.", Annotations: readOnlyHint}, listChannelMessages(client))
	mcp.AddTool(server, &mcp.Tool{Name: "teams_list_channel_message_replies", Description: "List replies to a root Teams channel message.", Annotations: readOnlyHint}, listChannelMessageReplies(client))
	mcp.AddTool(server, &mcp.Tool{Name: "teams_list_chats", Description: "List the target user's one-on-one, group, and meeting chats.", Annotations: readOnlyHint}, listChats(client))
	mcp.AddTool(server, &mcp.Tool{Name: "teams_get_chat", Description: "Get one Teams chat by id.", Annotations: readOnlyHint}, getChat(client))
	mcp.AddTool(server, &mcp.Tool{Name: "teams_list_chat_members", Description: "List members in an existing Teams chat.", Annotations: readOnlyHint}, listChatMembers(client))
	mcp.AddTool(server, &mcp.Tool{Name: "teams_search_messages", Description: "Search Teams channel and chat messages by free text.", Annotations: readOnlyHint}, searchMessages(client))
	mcp.AddTool(server, &mcp.Tool{Name: "teams_get_chat_message", Description: "Get a specific message in a Teams chat by id.", Annotations: readOnlyHint}, getChatMessage(client))
	mcp.AddTool(server, &mcp.Tool{Name: "teams_list_chat_messages", Description: "List messages in a Teams chat.", Annotations: readOnlyHint}, listChatMessages(client))
}
