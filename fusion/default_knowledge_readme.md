# MCPFusion Knowledge Store

You are connected to MCPFusion, an MCP server that provides access to APIs, services, and a persistent knowledge store. The knowledge store is your long-term memory — it persists across sessions and allows you to remember everything you learn about the user.

## Your Memory Tools

- **knowledge_get**: Retrieve entries. Use domain + key for a specific entry, domain alone to list a domain, or no parameters to list everything.
- **knowledge_set**: Store or update an entry. Entries are organized by domain (a category) and key (a unique identifier).
- **knowledge_delete**: Remove an entry you no longer need.

## How to Use This

1. **Read this file at the start of every session.** It tells you what you know about the user and which domains to consult before performing tasks.
2. **Proactively remember things.** When you learn something about the user — their preferences, how they like things done, important contacts, recurring tasks — store it. Don't wait to be asked.
3. **Update this readme as you learn.** When you create a new domain or learn something that should guide future sessions, use `knowledge_set` to update this entry (domain=`system`, key=`readme`) with new instructions. This file is your own bootstrap — make it better over time.
4. **Consult relevant domains before acting.** If the user asks you to check their email, and the "Domains to Consult" section below says to read the `email` domain first, do that before calling any email tools.

## Domains to Consult

_No domains configured yet. As you learn about the user, create domains and add a line here for each one. For example:_

- _`email` — Before accessing email, read this domain for known contacts, sorting rules, and preferences._
- _`calendar` — Before managing calendar events, read this domain for scheduling preferences._
