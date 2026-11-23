---
name: serena
description: "Use when semantic code navigation, symbol references, cross-file renames, or symbol-aware edits are needed in this workspace through the Serena MCP server."
---

# Serena

Use Serena for code tasks where understanding symbols and their relationships is more reliable than text-only search.

## Use Serena For

- Finding a symbol definition or all references across modules.
- Inspecting a file or project at the symbol level before changing it.
- Renaming Python symbols across files.
- Replacing, inserting, or safely deleting a function, class, or other symbol body.
- Checking project language-server diagnostics when the Serena tool is available.

## Workflow

1. Confirm that the `serena` MCP server is connected and that this workspace is the active project.
2. Read the relevant project or symbol instructions before making changes.
3. Prefer Serena's symbol and reference tools for semantic operations.
4. Use the built-in file and terminal tools for configuration files, non-code assets, tests, and commands that Serena does not own.
5. After an edit, run the narrowest relevant test, type check, or lint command.

## Guardrails

- Do not use a symbol rename when the requested change is only a string, environment variable, CLI argument, or serialized field rename.
- Inspect references before changing a public symbol or module API.
- Keep edits scoped to the requested behavior and preserve existing project conventions.
- Do not enable broad or destructive edits without first confirming the affected symbols and references.

## Setup

The workspace configuration launches Serena with `uvx` directly from `https://github.com/oraios/serena`, so Serena is not added to this application's runtime dependencies. Install `uv` in the environment that runs VS Code MCP servers if `uvx` is unavailable, then reload the MCP server from the Chat tools menu.