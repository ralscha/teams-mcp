// Package graph provides a focused Microsoft Graph client for the Teams
// endpoints exposed by teams-mcp.
package graph

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const maxResponseSize = 16 << 20

// Client is a Microsoft Graph client. Authentication is supplied by the
// configured HTTP client, normally created by internal/auth.
type Client struct {
	httpClient *http.Client
	baseURL    *url.URL
	userID     string
}

// NewClient creates a Graph client rooted at graphBaseURL, such as
// https://graph.microsoft.com/v1.0/. userID may be empty for delegated auth;
// otherwise /users/{userID} is used instead of /me for user-scoped endpoints.
func NewClient(graphBaseURL, userID string, httpClient *http.Client) (*Client, error) {
	u, err := url.Parse(graphBaseURL)
	if err != nil {
		return nil, fmt.Errorf("teams graph: invalid base URL: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("teams graph: base URL must be absolute")
	}
	if !strings.HasSuffix(u.Path, "/") {
		u.Path += "/"
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{httpClient: httpClient, baseURL: u, userID: userID}, nil
}

// APIError represents a non-2xx response from Microsoft Graph.
type APIError struct {
	StatusCode int
	Code       string
	Message    string
	RequestID  string
	RetryAfter string
}

func (e *APIError) Error() string {
	detail := e.Message
	if detail == "" {
		detail = http.StatusText(e.StatusCode)
	}
	if e.Code != "" {
		detail = e.Code + ": " + detail
	}
	return fmt.Sprintf("teams graph: request failed with status %d: %s", e.StatusCode, detail)
}

func (c *Client) doJSON(ctx context.Context, method, ref string, query url.Values, body, out any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("teams graph: encode request body: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := c.newRequest(ctx, method, ref, query, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("teams graph: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize+1))
	if err != nil {
		return fmt.Errorf("teams graph: read response body: %w", err)
	}
	if len(responseBody) > maxResponseSize {
		return fmt.Errorf("teams graph: response exceeds %d bytes", maxResponseSize)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return parseAPIError(resp, responseBody)
	}
	if out != nil && len(responseBody) > 0 {
		if err := json.Unmarshal(responseBody, out); err != nil {
			return fmt.Errorf("teams graph: decode response body: %w", err)
		}
	}
	return nil
}

func (c *Client) newRequest(ctx context.Context, method, ref string, query url.Values, body io.Reader) (*http.Request, error) {
	u, err := c.resolveURL(ref)
	if err != nil {
		return nil, err
	}
	if query != nil {
		u.RawQuery = query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, fmt.Errorf("teams graph: build request: %w", err)
	}
	return req, nil
}

// resolveURL supports Graph's opaque @odata.nextLink pagination URLs while
// refusing absolute URLs outside the configured Graph origin and API prefix.
func (c *Client) resolveURL(ref string) (*url.URL, error) {
	parsed, err := url.Parse(ref)
	if err != nil {
		return nil, fmt.Errorf("teams graph: invalid URL %q: %w", ref, err)
	}
	if parsed.IsAbs() {
		if parsed.User != nil {
			return nil, fmt.Errorf("teams graph: refusing URL containing user information")
		}
		if !strings.EqualFold(parsed.Scheme, c.baseURL.Scheme) || !strings.EqualFold(parsed.Host, c.baseURL.Host) {
			return nil, fmt.Errorf("teams graph: refusing pagination URL outside configured Graph origin")
		}
		if !strings.HasPrefix(parsed.EscapedPath(), c.baseURL.EscapedPath()) {
			return nil, fmt.Errorf("teams graph: refusing pagination URL outside configured Graph API prefix")
		}
		return parsed, nil
	}
	if strings.HasPrefix(ref, "/") {
		return nil, fmt.Errorf("teams graph: relative API paths must not start with a slash")
	}
	resolved := c.baseURL.ResolveReference(parsed)
	if !strings.HasPrefix(resolved.EscapedPath(), c.baseURL.EscapedPath()) {
		return nil, fmt.Errorf("teams graph: refusing relative URL outside configured Graph API prefix")
	}
	return resolved, nil
}

// pageRef validates that a caller-supplied next link targets the exact same
// Graph collection as the initial request. This prevents a tool argument from
// turning opaque pagination support into a general bearer-authenticated URL
// fetcher.
func (c *Client) pageRef(nextLink, initialRef string) (string, error) {
	if nextLink == "" {
		return initialRef, nil
	}
	parsed, err := url.Parse(nextLink)
	if err != nil {
		return "", fmt.Errorf("teams graph: invalid next link: %w", err)
	}
	if !parsed.IsAbs() {
		return "", fmt.Errorf("teams graph: next link must be an absolute Microsoft Graph URL")
	}
	next, err := c.resolveURL(nextLink)
	if err != nil {
		return "", err
	}
	initial, err := c.resolveURL(initialRef)
	if err != nil {
		return "", err
	}
	if next.EscapedPath() != initial.EscapedPath() {
		return "", fmt.Errorf("teams graph: next link does not match the requested collection")
	}
	return next.String(), nil
}

func (c *Client) userPath(suffix string) string {
	prefix := "me"
	if c.userID != "" {
		prefix = "users/" + url.PathEscape(c.userID)
	}
	if suffix == "" {
		return prefix
	}
	return prefix + "/" + suffix
}

func parseAPIError(resp *http.Response, body []byte) *APIError {
	apiErr := &APIError{
		StatusCode: resp.StatusCode,
		RequestID:  resp.Header.Get("request-id"),
		RetryAfter: resp.Header.Get("Retry-After"),
	}
	var payload struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			Inner   struct {
				RequestID string `json:"request-id"`
			} `json:"innerError"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err == nil {
		apiErr.Code = payload.Error.Code
		apiErr.Message = payload.Error.Message
		if apiErr.RequestID == "" {
			apiErr.RequestID = payload.Error.Inner.RequestID
		}
	}
	if apiErr.Message == "" && len(body) > 0 {
		apiErr.Message = strings.TrimSpace(string(body))
	}
	return apiErr
}
