package graph

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestRetriesRespectOperationAndServerDelay(t *testing.T) {
	for _, tc := range []struct {
		name       string
		method     string
		ref        string
		status     int
		retryAfter string
		attempts   int
		delay      time.Duration
	}{
		{"read throttled", http.MethodGet, "me", 429, "5", 2, 5 * time.Second},
		{"write throttled", http.MethodPost, "chats/c/messages", 429, "5", 2, 5 * time.Second},
		{"long throttle", http.MethodGet, "me", 429, "3600", 1, 0},
		{"read unavailable", http.MethodGet, "me", 503, "", 2, retryBaseDelay},
		{"read bad gateway", http.MethodGet, "me", 502, "", 2, retryBaseDelay},
		{"search timeout", http.MethodPost, "search/query", 504, "", 2, retryBaseDelay},
		{"send timeout", http.MethodPost, "chats/c/messages", 504, "", 1, 0},
		{"create unavailable", http.MethodPost, "chats", 503, "", 1, 0},
		{"edit unavailable", http.MethodPatch, "chats/c/messages/m", 503, "", 1, 0},
		{"delete unavailable", http.MethodPost, "chats/c/messages/m/softDelete", 503, "", 1, 0},
		{"forbidden", http.MethodGet, "me", 403, "", 1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			attempts := 0
			client := newTestClient(t, "", func(w http.ResponseWriter, r *http.Request) {
				attempts++
				if attempts == 1 {
					w.Header().Set("Retry-After", tc.retryAfter)
					w.WriteHeader(tc.status)
					_, _ = io.WriteString(w, `{"error":{"message":"try later"}}`)
					return
				}
				_, _ = io.WriteString(w, `{}`)
			})
			var waited time.Duration
			client.wait = func(_ context.Context, delay time.Duration) error {
				waited += delay
				return nil
			}
			err := client.doJSON(t.Context(), tc.method, tc.ref, nil, nil, nil)
			if attempts != tc.attempts || waited != tc.delay {
				t.Errorf("attempts=%d delay=%v, want %d / %v", attempts, waited, tc.attempts, tc.delay)
			}
			if (err != nil) != (tc.attempts == 1) {
				t.Errorf("unexpected result: %v", err)
			}
			if tc.retryAfter == "3600" && (err == nil || !strings.Contains(err.Error(), "Retry-After: 3600")) {
				t.Errorf("long throttle should expose the server delay: %v", err)
			}
		})
	}
}

func TestRetryCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	attempts := 0
	client := newTestClient(t, "", func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusTooManyRequests)
	})
	client.wait = func(ctx context.Context, delay time.Duration) error {
		cancel()
		return sleepContext(ctx, delay)
	}
	_, err := client.GetUser(ctx)
	if !errors.Is(err, context.Canceled) || attempts != 1 {
		t.Fatalf("attempts=%d, error=%v; want cancellation after one request", attempts, err)
	}
}

func TestRetryAfterOverflowAndHTTPDates(t *testing.T) {
	for _, value := range []string{"9223372036854775807", "18446744073709551616"} {
		delay, ok := parseRetryAfter(value)
		if !ok || delay != time.Duration(math.MaxInt64) {
			t.Fatalf("overflowing Retry-After %s = %v, %v", value, delay, ok)
		}
	}
	for _, layout := range []string{http.TimeFormat, time.RFC850, time.ANSIC} {
		delay, ok := parseRetryAfter(time.Now().UTC().Add(time.Minute).Format(layout))
		if !ok || delay < 58*time.Second || delay > time.Minute {
			t.Errorf("HTTP-date layout %q: delay=%v valid=%v", layout, delay, ok)
		}
	}
}

func TestGraphRedirectDoesNotForwardBearerToken(t *testing.T) {
	targetCalled := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetCalled = true
		_, _ = io.WriteString(w, `{}`)
	}))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("source request missing bearer token")
		}
		http.Redirect(w, r, target.URL+"/me", http.StatusTemporaryRedirect)
	}))
	defer source.Close()
	httpClient := oauth2.NewClient(t.Context(), oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"}))
	client, err := NewClient(source.URL+"/v1.0/", "", httpClient)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.GetUser(t.Context())
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusTemporaryRedirect || targetCalled {
		t.Errorf("redirect result=%v, targetCalled=%v", err, targetCalled)
	}
	if httpClient.CheckRedirect != nil {
		t.Error("NewClient modified the caller's HTTP client")
	}
}

func TestRejectsAmbiguousGraphURLs(t *testing.T) {
	client := newTestClient(t, "", func(w http.ResponseWriter, r *http.Request) {
		t.Error("invalid URL reached Graph")
	})
	for _, ref := range []string{
		"teams/../chats", "teams/%2e%2e/chats", "teams/./channels", "me#fragment",
		"//elsewhere.example/v1.0/me", client.baseURL.String() + "teams/../users",
		"chats/", "teams//channels", "chats/chat/messages/", "",
	} {
		if _, err := client.resolveURL(ref); err == nil {
			t.Errorf("accepted ambiguous URL %q", ref)
		}
	}
	if _, err := client.GetChannel(t.Context(), "..", "channel"); err == nil {
		t.Error("dot-segment team ID accepted")
	}
	for _, base := range []string{
		"ftp://graph.example/v1.0/", "https://user:secret@graph.example/v1.0/",
		"https://graph.example/v1.0/?x=1", "https://graph.example/v1.0/#fragment",
	} {
		if _, err := NewClient(base, "", nil); err == nil {
			t.Errorf("accepted ambiguous base URL %q", base)
		}
	}
}

func TestSendMentionIncludesZeroID(t *testing.T) {
	client := newTestClient(t, "", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Mentions []map[string]json.RawMessage `json:"mentions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		if len(body.Mentions) != 1 || string(body.Mentions[0]["id"]) != "0" {
			t.Errorf("wire mentions must include id:0: %v", body.Mentions)
		}
		_, _ = io.WriteString(w, `{"id":"sent"}`)
	})
	_, err := client.SendChatMessage(t.Context(), "chat", "html", `<at id="0">Ada</at>`, []ChatMessageMention{{
		ID: 0, MentionText: "Ada", Mentioned: MentionedIdentitySet{User: &Identity{ID: "ada"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateChatUsesSignedInUser(t *testing.T) {
	for _, tc := range []struct {
		name         string
		participants []string
		wantError    bool
	}{
		{"other user", []string{"other"}, false},
		{"deduplicate caller", []string{"CALLER", "other", "OTHER"}, false},
		{"self only", []string{"caller"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			posted := false
			client := newTestClient(t, "different-read-user", func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v1.0/me":
					_, _ = io.WriteString(w, `{"id":"caller"}`)
				case "/v1.0/chats":
					posted = true
					var request createChatRequest
					if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
						t.Error(err)
						return
					}
					if len(request.Members) != 2 || !strings.HasSuffix(request.Members[0].UserBind, "users('caller')") || !strings.HasSuffix(request.Members[1].UserBind, "users('other')") {
						t.Errorf("unexpected members: %+v", request.Members)
					}
					_, _ = io.WriteString(w, `{"id":"created"}`)
				default:
					t.Errorf("unexpected path %s", r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			})
			_, err := client.CreateChat(t.Context(), "oneOnOne", "", tc.participants)
			if (err != nil) != tc.wantError || posted == tc.wantError {
				t.Errorf("CreateChat() error=%v posted=%v", err, posted)
			}
		})
	}
}
