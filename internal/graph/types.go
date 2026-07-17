package graph

// User is the Microsoft Graph user targeted by this server.
type User struct {
	ID                string `json:"id,omitempty"`
	DisplayName       string `json:"displayName,omitempty"`
	UserPrincipalName string `json:"userPrincipalName,omitempty"`
	Mail              string `json:"mail,omitempty"`
}

// Team is the subset of Microsoft Graph team properties used by teams-mcp.
type Team struct {
	ID          string `json:"id,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	Description string `json:"description,omitempty"`
	IsArchived  bool   `json:"isArchived,omitempty"`
	TenantID    string `json:"tenantId,omitempty"`
}

// Channel is a Microsoft Teams channel.
type Channel struct {
	ID             string `json:"id,omitempty"`
	DisplayName    string `json:"displayName,omitempty"`
	Description    string `json:"description,omitempty"`
	Email          string `json:"email,omitempty"`
	MembershipType string `json:"membershipType,omitempty"`
	WebURL         string `json:"webUrl,omitempty"`
}

// Chat is a one-on-one, group, or meeting chat.
type Chat struct {
	ID                  string `json:"id,omitempty"`
	Topic               string `json:"topic,omitempty"`
	ChatType            string `json:"chatType,omitempty"`
	CreatedDateTime     string `json:"createdDateTime,omitempty"`
	LastUpdatedDateTime string `json:"lastUpdatedDateTime,omitempty"`
	WebURL              string `json:"webUrl,omitempty"`
	TenantID            string `json:"tenantId,omitempty"`
}

// ItemBody holds chatMessage content. ContentType is usually "text" or
// "html".
type ItemBody struct {
	ContentType string `json:"contentType,omitempty"`
	Content     string `json:"content,omitempty"`
}

// Identity describes a user, application, or device identity.
type Identity struct {
	ID               string `json:"id,omitempty"`
	DisplayName      string `json:"displayName,omitempty"`
	IdentityProvider string `json:"identityProvider,omitempty"`
}

// ChatMessageFrom identifies the sender of a message.
type ChatMessageFrom struct {
	User        *Identity `json:"user,omitempty"`
	Application *Identity `json:"application,omitempty"`
	Device      *Identity `json:"device,omitempty"`
}

// ChannelIdentity locates a message in a team and channel.
type ChannelIdentity struct {
	TeamID    string `json:"teamId,omitempty"`
	ChannelID string `json:"channelId,omitempty"`
}

// ChatMessageAttachment is attachment metadata included in a chat message.
type ChatMessageAttachment struct {
	ID           string `json:"id,omitempty"`
	ContentType  string `json:"contentType,omitempty"`
	ContentURL   string `json:"contentUrl,omitempty"`
	Name         string `json:"name,omitempty"`
	ThumbnailURL string `json:"thumbnailUrl,omitempty"`
}

// ChatMessage is a channel post, channel reply, or chat message.
type ChatMessage struct {
	ID                   string                  `json:"id,omitempty"`
	ReplyToID            string                  `json:"replyToId,omitempty"`
	ETag                 string                  `json:"etag,omitempty"`
	MessageType          string                  `json:"messageType,omitempty"`
	CreatedDateTime      string                  `json:"createdDateTime,omitempty"`
	LastModifiedDateTime string                  `json:"lastModifiedDateTime,omitempty"`
	DeletedDateTime      string                  `json:"deletedDateTime,omitempty"`
	Subject              string                  `json:"subject,omitempty"`
	Summary              string                  `json:"summary,omitempty"`
	ChatID               string                  `json:"chatId,omitempty"`
	WebURL               string                  `json:"webUrl,omitempty"`
	From                 *ChatMessageFrom        `json:"from,omitempty"`
	Body                 ItemBody                `json:"body"`
	ChannelIdentity      *ChannelIdentity        `json:"channelIdentity,omitempty"`
	Attachments          []ChatMessageAttachment `json:"attachments,omitempty"`
}

// Page is one page from a Microsoft Graph collection endpoint.
type Page[T any] struct {
	Value    []T    `json:"value"`
	NextLink string `json:"@odata.nextLink,omitempty"`
}
