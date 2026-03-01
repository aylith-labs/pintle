---
name: ship
description: Ship code changes with full validation and commit message based on actual changes
argument-hint: [staged]
---

# Ship Command

**CRITICAL**: This command enforces mandatory validation to prevent broken builds. Generates commit messages based on actual file changes.

## Usage

- `/ship` - Normal behavior, stages all changes with `git add -A`
- `/ship staged` - Skip staging, work with already staged changes

## Mandatory Validation Steps (MUST PASS)

1. **Lint Check** - `bun run lint`
   - Biome auto-fixes issues where possible
   - **Requirement**: Zero lint issues after auto-fix
   - **Unused Code Policy**:
     - NEVER use `biome check --unsafe` to suppress warnings
     - NEVER prepend variables with underscore (e.g., `_unusedVar`)
     - DELETE unused code immediately - remove unused functions, variables, imports

2. **Type Check** - `bun run typecheck`
   - **BLOCKS SHIP**: Any TypeScript errors
   - All TypeScript errors MUST be fixed before shipping
   - No exceptions: pre-existing errors also block the ship

3. **Unused Code Check** - `bun run find-unused`
   - **BLOCKS SHIP**: Any unused files, dependencies, or other issues
   - **AUTO-FIX**: If ONLY "Unused exports" and "Unused exported types" are found, run `bun run remove-unused` to auto-fix
   - **MANUAL REVIEW**: If knip reports unused files, dependencies, or other issues - STOP and ask user
   - Never auto-remove files - only safe to auto-remove unused exports/types

## Execution Order

### Phase 1: Parallel Validation (MANDATORY)

Run all 3 validation tasks in TRUE PARALLEL:

```
Bash("bun run lint")
Bash("bun run typecheck")
Bash("bun run find-unused")
```

**NEVER run these sequentially** - always use parallel tool calls.

### Phase 2: Change Analysis and Commit (After all validation passes)

1. **Stage changes conditionally:**
   - If `staged` flag: Skip staging, work with already staged changes
   - Otherwise: Stage all changes using `git add -A`

2. **Analyze staged changes:**
   - `git diff --name-status --staged`
   - `git diff --stat --staged`

3. **Generate and execute commit:**
   - Create commit message based on staged changes
   - Commit with the generated message
   - Push to remote

## Commit Message Style

This project uses plain descriptive messages (no conventional commit prefixes). Match this style:

```
Add Caddy label support for Docker auto-discovery
Configurable passthrough domains, local-proxy.* labels, single quotes
Add configurable BASE_DOMAIN env var (defaults to lvh.me)
Make port redirect scripts OS-agnostic (Linux + macOS)
Add settings menu with theme selection and font size controls
```

### Rules:
- Start with a verb (Add, Fix, Update, Remove, Refactor, etc.)
- Single line for small changes, multi-word descriptive summary
- NO conventional commit prefixes (no `feat:`, `fix:`, `chore:`, etc.)
- NO markdown formatting in commit messages
- NO ticket/issue references
- Analyze actual changes - don't use generic templates
- Be specific about what changed

## Failure Handling

**If ANY step fails, STOP immediately. DO NOT commit or push.**

- **Lint failures**: Fix by deleting unused code (never underscore prefixes). Re-run validation.
- **Type errors**: Fix all TypeScript errors. Re-run validation.
- **Unused code detected**: Delete unused exports/functions/variables/imports. Re-run validation.

## Safety

- Never commit hardcoded user-specific paths (`/home/username/`, `/Users/username/`)
- Use environment variables, `homedir()`, or relative paths instead
