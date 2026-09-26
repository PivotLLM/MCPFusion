# UnifyEM

`configs/unifyem.json` exposes the full [UnifyEM](https://github.com/UnifyEM/UnifyEM) administrator API as MCP tools. Everything `uem-cli` can do against the server is available; the few CLI features that run purely on the client (tag fan-out, waiting for a command result, local recovery-key generation and decryption) are handled by the assistant calling the corresponding tools in sequence.

## Setup

1. Set `UNIFYEM_URL` in `/opt/mcpfusion/env` to the server's external URL, for example `https://uem.example.com`.
2. Start MCPFusion with `-config configs/unifyem.json` (add it to your comma-separated list).
3. Destructive tools (see below) only run when MCPFusion is started with `MCP_FUSION_ALLOW_DESTRUCTIVE=true`.
4. If the UnifyEM server restricts `authorized_admin_ips`, the MCPFusion host must be on that list.

## Authentication

Each administrator authenticates as themselves. The first time a tool is called without credentials, MCPFusion returns a `fusion-auth` command. Running it prompts for the administrator's UnifyEM username and password, which MCPFusion stores against that administrator's MCPFusion API token. MCPFusion then logs in to UnifyEM's `/api/v1/login`, keeps the access token, and logs in again when it expires or when UnifyEM returns 401. UnifyEM's audit log therefore shows the real administrator for every action.

This requires the per-user `session_jwt` credentials feature described in `docs/proposal-per-user-session-jwt.md`. Until it ships, the `credentials` block in the auth config is ignored and the login body placeholders are sent literally, so authentication fails.

## Namespace

Tools are named `unifyem_<resource>_<action>` so a client can discover them progressively by prefix:

| Prefix | Scope |
|---|---|
| `unifyem_agent_*` | Agent records, triggers, tags, users, requests, recovery |
| `unifyem_cmd_*` | Commands queued to an agent (delivered on its next sync) |
| `unifyem_request_*` | Command request records and results |
| `unifyem_event_*` | Agent events |
| `unifyem_user_*` | UnifyEM user records (device users) |
| `unifyem_config_*` | Global agent config and server config |
| `unifyem_regtoken_*` | Agent registration token |
| `unifyem_report_*`, `unifyem_recovery_*`, `unifyem_deployfile_*`, `unifyem_ping` | Server-level functions |

Typical flow for running something on a device: `unifyem_agent_list` (or `unifyem_agent_list_by_tag`) to find an `agent_id`, a `unifyem_cmd_*` tool which returns a `request_id`, then `unifyem_request_get` until the status is `complete` or `failed`.

## Tools

Parameters marked `*` are required.

### Server

| Tool | Parameters | Description |
|---|---|---|
| `unifyem_ping` | — | Server reachability and credential check. |
| `unifyem_regtoken_get` | — | Current agent installation token. |
| `unifyem_regtoken_new` | — | Generate a new registration token; the old one stops working for new installs. |
| `unifyem_report_get` | `report`* (agents)<br>`format` (json, string) | Server-side report. |
| `unifyem_recovery_key_set` | `public_key`* | Upload the recovery public key agents encrypt recovery info with. |
| `unifyem_deployfile_create` | — | Regenerate `deploy.json` hashes after updating agent binaries. |

### Agents

| Tool | Parameters | Description |
|---|---|---|
| `unifyem_agent_list` | — | All agents with metadata, triggers, tags, users and last status. |
| `unifyem_agent_get` | `agent_id`* | One agent. |
| `unifyem_agent_list_by_tag` | `tag`* | Agents carrying a tag. |
| `unifyem_agent_rename` | `agent_id`*<br>`friendly_name`* | Set the friendly name. |
| `unifyem_agent_delete` | `agent_id`* | Delete the agent record. **Destructive.** |
| `unifyem_agent_trigger_lost` | `agent_id`* | Lost mode on next sync. **Destructive.** |
| `unifyem_agent_trigger_uninstall` | `agent_id`* | Agent uninstalls itself on next sync. **Destructive.** |
| `unifyem_agent_trigger_wipe` | `agent_id`* | Disk wipe on next sync. **Destructive.** |
| `unifyem_agent_trigger_reset` | `agent_id`* | Clear lost, uninstall and wipe triggers. |
| `unifyem_agent_tags_list` | `agent_id`* | Tags on an agent. |
| `unifyem_agent_tags_add` | `agent_id`*<br>`tags`* (array) | Add tags. |
| `unifyem_agent_tags_remove` | `agent_id`*<br>`tags`* (array) | Remove tags. |
| `unifyem_agent_users_add` | `agent_id`*<br>`users`* (array) | Associate UnifyEM users with a device. |
| `unifyem_agent_users_remove` | `agent_id`*<br>`users`* (array) | Remove the association. |
| `unifyem_agent_requests_list` | `agent_id`* | Command history for one agent. |
| `unifyem_agent_requests_cancel` | `agent_id`* | Cancel all pending requests for an agent. |
| `unifyem_agent_recovery_get` | `agent_id`* | Encrypted recovery blob, or empty if none reported. |
| `unifyem_event_list` | `agent_id`*<br>`type` (message, alert, status)<br>`start` (YYYYMMDD)<br>`end` (YYYYMMDD)<br>`start_time` (unix)<br>`end_time` (unix) | Agent events. |

### Commands

All command tools take `agent_id`*, queue the command via `POST /api/v1/cmd` and return a `request_id`.

| Tool | Additional parameters | Description |
|---|---|---|
| `unifyem_cmd_ping` | — | Ask the agent to acknowledge. |
| `unifyem_cmd_status` | — | Request a fresh status report. |
| `unifyem_cmd_execute` | `cmd`*<br>`arg1` … `arg12`<br>`ssh` | Run a program or shell command. |
| `unifyem_cmd_download_execute` | `url`*<br>`hash`<br>`arg1` … `arg12` | Download and run a file. |
| `unifyem_cmd_upgrade` | — | Agent self-upgrade from the server. |
| `unifyem_cmd_refresh_service_account` | — | Rotate the agent's service account password. |
| `unifyem_cmd_reboot` | — | Reboot the endpoint. |
| `unifyem_cmd_shutdown` | — | Shut the endpoint down. |
| `unifyem_cmd_user_list` | — | List local OS accounts. |
| `unifyem_cmd_user_add` | `user`*<br>`password`*<br>`admin` | Create a local OS account. |
| `unifyem_cmd_user_delete` | `user`*<br>`shutdown` | Delete a local OS account. **Destructive.** |
| `unifyem_cmd_user_admin` | `user`*<br>`admin`* | Grant or revoke local admin. |
| `unifyem_cmd_user_password` | `user`*<br>`password`* | Change a local account password. |
| `unifyem_cmd_user_lock` | `user`*<br>`shutdown` | Lock a local account. |
| `unifyem_cmd_user_unlock` | `user`*<br>`password`* | Unlock a local account. |

### Requests

| Tool | Parameters | Description |
|---|---|---|
| `unifyem_request_list` | — | All request records. |
| `unifyem_request_get` | `request_id`* | One request with status and result. |
| `unifyem_request_cancel` | `request_id`* | Cancel a pending request. |
| `unifyem_request_delete` | `request_id`* | Delete a request record. **Destructive.** |

### Users

| Tool | Parameters | Description |
|---|---|---|
| `unifyem_user_list` | — | UnifyEM user records. |
| `unifyem_user_get` | `user_id`* | One user record. |
| `unifyem_user_add` | `user`*<br>`email`*<br>`display_name` | Create a user record. |
| `unifyem_user_delete` | `user_id`* | Delete a user record. **Destructive.** |

### Configuration

| Tool | Parameters | Description |
|---|---|---|
| `unifyem_config_agents_get` | — | Global agent configuration. |
| `unifyem_config_agents_set` | `parameters`* (object) | Update existing agent config keys. |
| `unifyem_config_server_get` | — | Server configuration. |
| `unifyem_config_server_set` | `parameters`* (object) | Update existing server config keys. |

## Hints

Every tool carries `openWorld: true`. GET endpoints and `report_get` carry `readOnly: true`. Destructive gating applies to the three DELETE endpoints automatically and to lost, uninstall, wipe and OS `user_delete` explicitly, judged by impact on the device's user. Reboot and shutdown are not gated because the user can wait for the device to come back or power it on.

Copyright (c) 2025-2026 Tenebris Technologies Inc. See LICENSE for details.
