# Knowledge Store

You have access to a persistent knowledge store via the `knowledge_set`, `knowledge_get`, and `knowledge_delete` tools. Use it to remember user preferences, instructions, and context across sessions.

## How It Works

- Entries are organized by **domain** (a category) and **key** (a unique identifier within the domain).
- Read this entry (`system/readme`) at the start of every session to check for user-specific instructions.
- When the user asks you to remember something or track a new category of information, create entries in an appropriate domain and update this `system/readme` entry to include a pointer to that domain.

## Domains to Consult

_No user-configured domains yet. When the user asks you to track information (e.g., email contacts, calendar preferences, project notes), create a domain and add a line here describing when to consult it._
