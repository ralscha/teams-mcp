package graph

import (
	"context"
	"net/http"
	"net/url"
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

// ListJoinedTeams returns the teams that the target user directly belongs to.
func (c *Client) ListJoinedTeams(ctx context.Context) (*Page[Team], error) {
	var page Page[Team]
	if err := c.doJSON(ctx, http.MethodGet, c.userPath("joinedTeams"), nil, nil, &page); err != nil {
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
