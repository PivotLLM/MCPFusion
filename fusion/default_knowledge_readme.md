# MCPFusion Knowledge Store

Read this entry at the start of every session. It is an index — keep it that way.

You are connected to MCPFusion, an MCP server that provides access to APIs, services, and a persistent knowledge store. The knowledge store is your long-term memory — it persists across sessions and allows you to remember everything you learn about the user.

## Rules for This Readme

- **This entry is an index only.** It points to where detailed instructions live. Never put detailed instructions, preferences, or data directly in this entry.
- **Keep it lightweight.** Each task area gets one line pointing to its detailed instructions entry. If you need to add new guidance, create a new `system/<topic>-instructions` entry and add a one-line pointer here.
- **Update the index when you create new domains or instruction entries.** Future sessions depend on this being current.
- **You may update this entry.** To update this index, use `knowledge_set(domain="system", key="readme", content=...)`. Include all existing content plus your additions.

## Memory Tools

- `knowledge_get(domain, key)` — read a specific entry, list a domain, or list everything
- `knowledge_set(domain, key, content)` — store or update an entry
- `knowledge_delete(domain, key)` — remove an entry
- `knowledge_rename(domain, old_key, new_key)` — rename an entry's key within the same domain
- `knowledge_search(query)` — case-insensitive search across all domains, keys, and content

## Principles

1. Proactively remember things you learn about the user — don't wait to be asked.
2. Consult task-specific instructions below before performing tasks.
3. Key entries by natural lookup values (e.g., email entries keyed by domain name like `email/example.com`).
4. Use `system/<topic>-instructions` entries for detailed handling rules. Keep data entries (contacts, preferences) in their own domains.

## Task-Specific Instructions

_No instructions configured yet. As you learn about the user's workflows, create `system/<topic>-instructions` entries and add a one-line pointer here. Remove these examples once real instructions are added._

- _**Email**: Read `system/email-instructions` before accessing, reading, or summarizing email._
- _**Calendar**: Read `system/calendar-instructions` before managing calendar events._

