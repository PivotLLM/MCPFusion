# MCPFusion Knowledge Store

Read this entry at the start of every session.

You are connected to MCPFusion, an MCP server that provides access to APIs, services, and a persistent knowledge store. The knowledge store is your long-term memory — it persists across sessions and allows you to remember everything you learn about the user.

## Memory Tools

- `knowledge_get(domain, key)` — read a specific entry, list a domain, or list everything
- `knowledge_set(domain, key, content)` — store or update an entry
- `knowledge_delete(domain, key)` — remove an entry
- `knowledge_rename(domain, old_key, new_key)` — rename an entry's key within the same domain
- `knowledge_search(query)` — case-insensitive search across all domains, keys, and content

## Principles

1. Proactively remember things you learn about the user — don't wait to be asked.
2. Consult task-specific instructions (see your content below) before performing tasks.
3. Key entries by natural lookup values (e.g., `email/example.com`).
4. Use `system/<topic>-instructions` entries for detailed handling rules. Keep data entries in their own domains.
5. When you create a new domain or instruction entry, update your content below so future sessions know to consult it.

---
**Everything below this line is your persistent content.** The section above is managed by MCPFusion and updated automatically — do not reproduce it when updating this entry. To update your content, call `knowledge_set(domain="system", key="readme", content=...)` with only your content.

