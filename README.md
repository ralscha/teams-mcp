# teams-mcp

A [Model Context Protocol](https://modelcontextprotocol.io/) server that
connects MCP clients to Microsoft Teams through
[Microsoft Graph](https://learn.microsoft.com/graph/teams-concept-overview).
It follows the same small Go project shape as `jira-mcp` and uses the official
[`github.com/modelcontextprotocol/go-sdk`](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp).

The server supports **stdio** and **streamable HTTP**, defaults to a safe
**readonly** mode, and can authenticate with delegated device-code OAuth,
client credentials, or an existing Graph access token.

The scope is intentionally focused: it discovers teams, channels, and chats;
reads their messages; and, when explicitly enabled, sends messages. It is not
intended to expose every Microsoft Teams or Microsoft Graph operation.

## Installation

Download the latest release for your platform from this repository's
**Releases** page. Release archives contain the binary, README, and license;
`checksums.txt` and a GitHub artifact attestation are published alongside
them. You can also build from source:

```bash
go build ./cmd/teams-mcp
```

Confirm which build is installed with `teams-mcp --version`.

## Tools

Read tools are always available:

| Tool | Description |
| --- | --- |
| `teams_get_current_user` | Get the Microsoft 365 user whose Teams data is being accessed |
| `teams_get_user` | Get one Microsoft 365 user by object id or UPN |
| `teams_list_users` | List Microsoft 365 users with optional prefix filtering |
| `teams_list_joined_teams` | List teams the user directly belongs to |
| `teams_list_channels` | List channels in a team |
| `teams_get_channel` | Get one channel by id |
| `teams_list_team_members` | List members in a team |
| `teams_get_channel_message` | Get a specific root channel message by id |
| `teams_list_channel_messages` | List root messages in a channel |
| `teams_list_channel_message_replies` | List replies to a root channel message |
| `teams_list_chats` | List one-on-one, group, and meeting chats, including participant summaries when available |
| `teams_get_chat` | Get one chat by id |
| `teams_list_chat_members` | List members in a chat |
| `teams_search_messages` | Search channel and chat messages by free text |
| `teams_get_chat_message` | Get a specific chat message by id |
| `teams_list_chat_messages` | List messages in a chat |

Write tools are exposed only with `TEAMS_MODE=readwrite`:

| Tool | Description |
| --- | --- |
| `teams_create_chat` | Create a one-on-one or group chat |
| `teams_send_channel_message` | Send a root channel message |
| `teams_update_channel_message` | Edit an existing root channel message |
| `teams_delete_channel_message` | Soft-delete an existing root channel message |
| `teams_reply_to_channel_message` | Reply to a root channel message |
| `teams_send_chat_message` | Send a message to an existing chat |
| `teams_update_chat_message` | Edit an existing chat message |
| `teams_delete_chat_message` | Soft-delete an existing chat message |

When a message includes mentions, read tools return a structured `mentions`
array with mentioned identity details. Write tools accept an optional
`mentions` array; when provided, set `content_type` to `html` and include
matching `<at id="N">...</at>` tags in message HTML.

Paginated collection tools return Graph's opaque `next_link` when another
page exists. Pass it back unchanged on the next call. Message bodies are
returned in the `text` or `html` form supplied by Teams. Treat message HTML as
untrusted content and sanitize it before rendering it outside an MCP client.

## Recommended authentication: device code

Device-code OAuth signs in a work or school user and caches the refresh token
outside the repository. It is the recommended method because normal Graph
message sends require delegated user permissions.

### 1. Register a Microsoft Entra application

1. In the [Microsoft Entra admin center](https://entra.microsoft.com/), create
   an app registration.
2. Under **Authentication**, enable **Allow public client flows**.
3. Add these **delegated** Microsoft Graph permissions for readonly mode:
   `User.Read`, `User.ReadBasic.All`, `Team.ReadBasic.All`,
   `TeamMember.Read.All`, `Channel.ReadBasic.All`, `ChannelMessage.Read.All`,
   `Chat.Read`, and `ChatMember.Read`.
4. For readwrite mode also add `ChannelMessage.Send`,
   `ChannelMessage.ReadWrite`, `ChatMessage.Send`, `Chat.ReadWrite`, and
   `Chat.Create`. Editing and soft-deleting messages requires the
   `ReadWrite` scopes; sending alone is not sufficient.
5. Grant tenant admin consent where your tenant requires it. In particular,
   `ChannelMessage.Read.All` requires admin consent.

Microsoft documents the relevant permissions on the
[joined teams](https://learn.microsoft.com/graph/api/user-list-joinedteams?view=graph-rest-1.0),
[channel list](https://learn.microsoft.com/graph/api/channel-list?view=graph-rest-1.0),
[channel message list](https://learn.microsoft.com/graph/api/channel-list-messages?view=graph-rest-1.0),
[chat list](https://learn.microsoft.com/graph/api/chat-list?view=graph-rest-1.0), and
[chat message list](https://learn.microsoft.com/graph/api/chat-list-messages?view=graph-rest-1.0)
API pages.

### 2. Run the server

```bash
export TEAMS_TENANT_ID=00000000-0000-0000-0000-000000000000
export TEAMS_CLIENT_ID=11111111-1111-1111-1111-111111111111

go run ./cmd/teams-mcp --mode=readonly --transport=stdio
```

On the first run, `teams-mcp` writes a Microsoft sign-in URL and user code to
stderr. Complete that sign-in once. The token is cached at the operating
system's user cache location (for example
`~/.cache/teams-mcp/token.json` on Linux). Set `TEAMS_TOKEN_CACHE` to choose a
different path, or pass `--teams-token-cache=` to disable persistence.

The cache contains Microsoft OAuth tokens. Do not commit, sync, or share it,
and keep its directory accessible only to the account running `teams-mcp`.

The default delegated scopes depend on the mode. Set `TEAMS_SCOPES` only when
you need to override them.

## Other authentication methods

### Existing access token

Set `TEAMS_ACCESS_TOKEN` or `--teams-access-token`. Auto-detection selects the
`access-token` method when a token is present. The server cannot refresh a
token supplied this way.

```bash
TEAMS_ACCESS_TOKEN=ey... teams-mcp --mode=readonly
```

For readwrite mode, the token must be delegated and include
`ChannelMessage.Send` and/or `ChatMessage.Send`. Microsoft Graph only supports
ordinary [channel sends](https://learn.microsoft.com/graph/api/channel-post-messages?view=graph-rest-1.0)
and [chat sends](https://learn.microsoft.com/graph/api/chat-post-messages?view=graph-rest-1.0)
with delegated permissions; its documented app-only permission is intended for
migration.

### Client credentials (app-only reads)

Set `TEAMS_TENANT_ID`, `TEAMS_CLIENT_ID`, `TEAMS_CLIENT_SECRET`, and
`TEAMS_USER_ID`. `TEAMS_USER_ID` can be an object ID or user principal name and
selects the user whose teams and chats are read.

Grant the application the Graph **application** permissions needed by the
tools you use, normally `User.Read.All`, `Team.ReadBasic.All`,
`Channel.ReadBasic.All`, `ChannelMessage.Read.All`, and `Chat.Read.All`, then
grant admin consent. Client-credentials auth is intentionally rejected in
readwrite mode because it cannot perform normal Teams message sends.

## Configuration

Flags override environment variables. When `TEAMS_AUTH_METHOD=auto`, an access
token wins, then a client secret, otherwise device code is selected.

| Environment variable | CLI flag | Default | Description |
| --- | --- | --- | --- |
| `TEAMS_TENANT_ID` | `--teams-tenant-id` | | Entra tenant ID or verified domain |
| `TEAMS_CLIENT_ID` | `--teams-client-id` | | Entra application/client ID |
| `TEAMS_CLIENT_SECRET` | `--teams-client-secret` | | Secret for client-credentials auth |
| `TEAMS_ACCESS_TOKEN` | `--teams-access-token` | | Pre-acquired Graph token |
| `TEAMS_USER_ID` | `--teams-user-id` | | Target object ID or UPN; required for app-only auth |
| `TEAMS_AUTH_METHOD` | `--auth-method` | `auto` | `auto`, `device-code`, `client-credentials`, or `access-token` |
| `TEAMS_SCOPES` | `--teams-scopes` | mode-dependent | Space- or comma-separated delegated scopes |
| `TEAMS_TOKEN_CACHE` | `--teams-token-cache` | OS user cache | Device-code token cache path |
| `TEAMS_GRAPH_BASE_URL` | `--teams-graph-base-url` | `https://graph.microsoft.com/v1.0/` | Graph API base, useful for national clouds |
| `TEAMS_AUTHORITY_URL` | `--teams-authority-url` | `https://login.microsoftonline.com/` | Microsoft identity authority base |
| `TEAMS_MODE` | `--mode` | `readonly` | `readonly` or `readwrite` |
| `MCP_TRANSPORT` | `--transport` | `stdio` | `stdio` or `http` |
| `MCP_HTTP_ADDR` | `--http-addr` | `:8080` | HTTP listen address |

## MCP client configuration

For Claude Desktop or another stdio host:

```json
{
  "mcpServers": {
    "teams": {
      "command": "teams-mcp",
      "args": [],
      "env": {
        "TEAMS_TENANT_ID": "00000000-0000-0000-0000-000000000000",
        "TEAMS_CLIENT_ID": "11111111-1111-1111-1111-111111111111",
        "TEAMS_MODE": "readonly"
      }
    }
  }
}
```

For VS Code, put the same server object under `servers` in `.vscode/mcp.json`.
It is usually easiest to run the binary once in a terminal to complete the
initial device-code sign-in before launching it from an MCP host.

## Transports and security

The default stdio transport communicates over stdin/stdout:

```bash
teams-mcp --transport=stdio
```

Streamable HTTP listens on the configured address:

```bash
teams-mcp --transport=http --http-addr=127.0.0.1:8080
```

The HTTP transport has no built-in client authentication. Bind to loopback or
protect it with an authenticated reverse proxy and firewall, especially in
readwrite mode. All MCP clients connected to one server process share that
process's configured Microsoft identity.

## Development

Requires Go 1.26.5 or newer. The full local check also requires Docker,
[Task](https://taskfile.dev/), and [GoReleaser](https://goreleaser.com/).

```bash
go build ./...
go test ./...
```

Using Task:

```bash
task build
task format
task test
task vet
task lint
task release-check
task check
```

`task check` runs the build, tests, `go vet`, the pinned golangci-lint Docker
image, and GoReleaser configuration validation.

## Release

This repository includes GitHub Actions workflows for test and release:

- `.github/workflows/test.yml` tests Linux, macOS, and Windows, and separately
  checks formatting, module tidiness, `go vet`, race-enabled tests,
  golangci-lint, and a complete GoReleaser snapshot.
- `.github/workflows/release.yml` reruns the tests, publishes GoReleaser
  artifacts when a `v*` tag is pushed, and creates a GitHub artifact
  attestation from `checksums.txt`.

To create and push a release tag locally:

```bash
task release-tag TAG=v0.1.0
```

Or use the split flow:

```bash
task tag TAG=v0.1.0
task push-tag TAG=v0.1.0
```

GoReleaser is configured in `.goreleaser.yaml` to build `teams-mcp` for Linux,
macOS, and Windows on `amd64` and `arm64`, publish archives, and attach
`checksums.txt` to the GitHub release. It also injects the tag version into the
binary and MCP server metadata, trims local build paths, and uses the commit
timestamp for reproducible build metadata.

Tests cover configuration and mode gating, token handling, all Graph endpoint
families against local HTTP mocks, pagination URL validation, Graph error
mapping, and end-to-end MCP smoke tests over stdio and streamable HTTP.

## License

MIT — see [LICENSE](LICENSE).
