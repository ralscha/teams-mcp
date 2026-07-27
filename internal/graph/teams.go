package graph

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// GetUser returns the delegated user (/me) or configured target user.
func (c *Client) GetUser(ctx context.Context) (*User, error) {
	query := url.Values{}
	query.Set("$select", "id,displayName,userPrincipalName,mail")
	var user User
	if err := c.doJSON(ctx, http.MethodGet, c.userPath(""), query, nil, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByID returns a specific user by object id or UPN.
func (c *Client) GetUserByID(ctx context.Context, userID string) (*User, error) {
	trimmed := strings.TrimSpace(userID)
	if trimmed == "" {
		return nil, fmt.Errorf("user id must not be empty")
	}
	query := url.Values{}
	query.Set("$select", "id,displayName,userPrincipalName,mail")
	var user User
	if err := c.doJSON(ctx, http.MethodGet, "users/"+url.PathEscape(trimmed), query, nil, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// ListJoinedTeams returns one page of teams that the target user directly
// belongs to. Pass the previous response's nextLink to retrieve the next
// page.
func (c *Client) ListJoinedTeams(ctx context.Context, nextLink string) (*Page[Team], error) {
	initialRef := c.userPath("joinedTeams")
	ref, err := c.pageRef(nextLink, initialRef)
	if err != nil {
		return nil, err
	}
	var page Page[Team]
	if err := c.doJSON(ctx, http.MethodGet, ref, nil, nil, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

// ListChannels returns one page of channels in a team. Pass the previous
// response's nextLink to retrieve the next page.
func (c *Client) ListChannels(ctx context.Context, teamID, nextLink string) (*Page[Channel], error) {
	initialRef := "teams/" + url.PathEscape(teamID) + "/channels"
	ref, err := c.pageRef(nextLink, initialRef)
	if err != nil {
		return nil, err
	}
	var query url.Values
	if nextLink == "" {
		// Excluding email avoids an expensive Graph lookup that this server does
		// not expose. Pagination URLs already carry Graph's selected fields.
		query = url.Values{}
		query.Set("$select", "id,displayName,description,membershipType,webUrl")
	}
	var page Page[Channel]
	if err := c.doJSON(ctx, http.MethodGet, ref, query, nil, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

// GetChannel returns one channel in a team.
func (c *Client) GetChannel(ctx context.Context, teamID, channelID string) (*Channel, error) {
	query := url.Values{}
	query.Set("$select", "id,displayName,description,membershipType,webUrl")
	ref := "teams/" + url.PathEscape(teamID) + "/channels/" + url.PathEscape(channelID)
	var channel Channel
	if err := c.doJSON(ctx, http.MethodGet, ref, query, nil, &channel); err != nil {
		return nil, err
	}
	return &channel, nil
}

// ListUsers returns one page of users from Microsoft Entra. When query is
// set, displayName, mail, and userPrincipalName are prefix-filtered.
func (c *Client) ListUsers(ctx context.Context, query, nextLink string, top int) (*Page[User], error) {
	initialRef := "users"
	ref, err := c.pageRef(nextLink, initialRef)
	if err != nil {
		return nil, err
	}
	var params url.Values
	if nextLink == "" {
		params = collectionQuery(top)
		if params == nil {
			params = url.Values{}
		}
		params.Set("$select", "id,displayName,userPrincipalName,mail")
		trimmed := strings.TrimSpace(query)
		if trimmed != "" {
			escaped := strings.ReplaceAll(trimmed, "'", "''")
			params.Set("$filter", "startswith(displayName,'"+escaped+"') or startswith(mail,'"+escaped+"') or startswith(userPrincipalName,'"+escaped+"')")
		}
	}
	var page Page[User]
	if err := c.doJSON(ctx, http.MethodGet, ref, params, nil, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

// ListTeamMembers returns members in a Microsoft Team.
func (c *Client) ListTeamMembers(ctx context.Context, teamID, nextLink string, top int) (*Page[ConversationMember], error) {
	initialRef := "teams/" + url.PathEscape(teamID) + "/members"
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
