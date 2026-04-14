# AGENTS.md

Project instructions for Codex in this repository.

## Project Facts

- This repo is YourPlace, a distributed social media platform.
- Main layers:
  - `src/core/`: business logic, database, blockchain, networking, security
  - `src/routes/`: HTTP handlers and API endpoints
  - `src/templates/`: server-rendered Go templates
  - `src/typescript/` and `src/scss/`: frontend behavior and styling
- Main architectural patterns:
  - wallet-based authentication
  - multi-chain support, with Base primary and Algorand and Ethereum also supported
  - embedded IPFS for decentralized content storage
  - SQLite and MySQL support through shared database abstractions
- Important locations:
  - `src/core/db/`
  - `src/templates/components/`
  - `src/typescript/components/`
  - `src/typescript/util/`
  - `src/scss/components/modalDialog.scss`
  - `docs/notes/StyleGuide.md`

## Common Commands

- Clean build: `make clean install build`
- Debug build: `make clean install dbg_build dbg_run`
- Tests: `make test`
- Helper-only build: `make helper_build`
- Install frontend dependencies: `npm install`
- Frontend production bundle: `npx webpack --config src/typescript/webpack.prod.js`
- Frontend development bundle: `npx webpack --config src/typescript/webpack.dev.js`
- Runtime flags:
  - `-d` debug mode
  - `-g` gateway mode
  - `-du` disable UI auto-open
  - `-di` disable indexer
  - `-c=<64-char-hex>` crypto seed
- Environment variable:
  - `YOURPLACE_MYSQL_DSN`

## Must

- Read the relevant code before editing it. Do not change code you have not read.
- Keep changes minimal, readable, and consistent with the existing architecture.
- Match the style of nearby files in the same directory, including naming, ordering, spacing, compactness, newline density, and general structure.
- Prefer existing helpers in `src/core/`, `src/typescript/components/`, and `src/typescript/util/` before introducing new abstractions.
- Treat all user input, external input, RPC data, and on-chain data as untrusted until validated.
- Use existing security functions when handling user input, external systems, database access, or blockchain data.
- Use parameterized SQL only.
- Preserve the original case of on-chain data such as wallet addresses and ENS or NFD-style names. Keep comparisons and lookups case-sensitive where appropriate.
- Keep blockchain-specific logic inside the appropriate abstraction or chain-specific files such as `wallet.go`, `wallet.ts`, `base.*`, `ethereum.*`, and related chain modules.
- Keep each blockchain indexer isolated to its own chain, except for clearly chain-agnostic shared helpers that already exist.
- When changing database schema, update all supported database implementations in `src/core/db/` and keep `schema.go` and engine-specific code aligned.
- When changing frontend styles, prefer scoped changes over global ones.
- In TypeScript files, keep the `DOM = {}` structure near the top of the execution flow, before functions.
- Keep declarations organized near the top of files: imports, then types and variables, then functions, unless a narrower local declaration is clearly better.
- Keep indentation aligned with surrounding code.
- For snapshot-related work, prefer modifying the snapshot service rather than reshaping core server flow around it.
- For long-form modal content, use `<div class='modalBodyLeft'>...</div>` instead of changing the global modal alignment.
- Treat `704px` as the mobile versus desktop breakpoint. Treat `360px` to `704px` as mobile and anything above `704px` as desktop.
- Perform source-to-sink analysis when doing vulnerability work. Verify exploitability through actual data flow.

## Must Not

- Do not broaden scope beyond the requested task.
- Do not delete or refactor unrelated code unless the task explicitly requires it.
- Do not introduce unsafe XSS sinks without appropriate built-in protection.
- Do not scatter blockchain-specific strings or branching logic across unrelated files.
- Do not co-mingle blockchain logic across chains in `*_indexer.go` files.
- Do not use non-parameterized SQL.
- Do not use hyphens in multi-word Make targets. Use underscores instead.
- Do not add inline styles unless there is no reasonable alternative.
- Do not use `!important` unless there is no reasonable alternative.
- Do not make global styling changes when a local change will solve the problem.
- Do not recolor unrelated global UI controls when adjusting a user's profile colors.
- Do not flip the default centered modal alignment globally.
- Do not add comments unless they are clearly necessary or explicitly requested.

## Prefer

- Prefer project-native implementations over new third-party libraries unless the dependency is already established in the repo.
- Prefer business-specific helpers to live near their usage. Prefer shared utility files only for genuinely reusable logic.
- Prefer `LogDebug*` for Go logging.
- Prefer `LogError*` only for real code errors that need investigation.
- Prefer not to use `Printf`-style logging.
- Prefer not to use `LogInfo*` unless specifically requested.
- Prefer alphabetical ordering inside obvious list-like blocks when that improves consistency.
- Prefer CSS classes in SCSS over inline HTML styling.
- Prefer modifying modal content for a single use case instead of modifying the shared modal component globally.
- Prefer keeping profile color changes limited to profile-specific UI such as the profile page, profile cards, post cards, and obvious profile-local controls.
- Prefer the future-safe migration pattern for large tables: add column without default, backfill in batches, then add the default.
- Prefer using `ENS` as the generic internal code term for on-chain naming systems when the existing codebase already does that, even if a specific chain uses another public-facing name such as `NFD`.

## If Unsure

- Follow explicit user instructions over preferences.
- Follow `Must` and `Must Not` rules unless the user explicitly asks to override them.
- If a requested change appears to conflict with the blockchain isolation, security, or migration rules above, pause and ask before proceeding.
