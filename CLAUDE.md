# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Common Development Commands

### Building and Running
- **Clean build**: `make clean install build`
- **Debug build**: `make clean install dbg_build dbg_run`
- **Run tests**: `make test` (requires golangci-lint)
- **Helper only build**: `make helper_build`

### Frontend Development
- **Install dependencies**: `npm install` (or `npm.cmd` on Windows)
- **Production build**: `npx webpack --config src/typescript/webpack.prod.js`
- **Development build**: `npx webpack --config src/typescript/webpack.dev.js`

### Runtime Flags
- **Debug mode**: `-d` (also creates ~/YourPlace/debug file)
- **Gateway mode**: `-g` (for distributed deployments \[typically on cloud providers to serve the public internet\])  
- **Disable UI**: `-du` (don't auto-open browser)
- **Disable indexer**: `-di` (or create noindexer file)
- **Crypto seed**: `-c=<64-char-hex>` (for distributed crypto state)

### Environment Variables
- **YOURPLACE_MYSQL_DSN**: sets the back end MySQL-compatible database server (<username>:<password>@tcp(<hostname>:<port>)/<database>)

### Platform-Specific Notes
- Windows builds create UPX-compressed binaries in `target/YourPlace-<version>.exe`
- macOS builds create `.pkg` installers via `resources/osx/osx_packager.sh`
- Helper binaries are platform-specific and embedded into the main binary

## Architecture Overview

### Core Application Structure
YourPlace is a **distributed social media platform** built with a 4-tier layered architecture:

1. **Core Layer** (`src/core/`) - Business logic, database, blockchain, networking, security
2. **Routes Layer** (`src/routes/`) - HTTP handlers and API endpoints
3. **Templates Layer** (`src/templates/`) - Server-side HTML rendering with Go templates
4. **Frontend Layer** (`src/typescript/`, `src/scss/`) - Client-side TypeScript and styling

### Key Architectural Patterns

#### **Blockchain-Native Identity**
- Users authenticate via wallet signatures (MetaMask, WalletConnect, Sign-In-With-Ethereum, etc.)
- Posts and content stored on-chain via post transactions and/or smart contracts
- Multi-blockchain support: Base (primary), Algorand, Ethereum
- Blockchain indexer processes on-chain data into local database (SQLite or MySQL)

#### **Decentralized Storage**
- IPFS node embedded for content storage (port 42425 by default)
- Content-addressed storage with cryptographic hashing
- Files pinned locally and distributed across the network
- BadBits denylist integration for content moderation

#### **Security Model**
- AES-256-GCM encryption for sensitive data with configurable crypto seed
- CSRF protection via middleware with secure token generation
- Rate limiting and content validation on all endpoints
- Loopback-only middleware for local-first operation
- Noncustodial model with users holding / generating wallet keys locally

#### **Database Architecture**
- SQLite & MySQL support with a pluggable database interface
- Supports user profiles, posts, blockchain transactions, file metadata
- Built-in migration system and default value seeding
- Blockchain indexer populates the database from on-chain events

### Development Workflow

#### **Frontend Changes**
1. Edit TypeScript files in `src/typescript/`
2. Edit SCSS files in `src/scss/`
3. Edit HTML files in `src/templates/`
4. Public static files are stored in `src/www/` which maps to the HTTP endpoint `/static/`
5. Run webpack build to compiled assets in `src/www/js/`

#### **Backend Changes**
1. Edit Go files in `src/core/`, `src/routes/`, or `main.go`
2. Ensure database schema changes include migrations from `schema.go`
3. Run `make test` to verify with golangci-lint
4. Test with debug build before production

#### **Template Changes**
1. Edit `.tmpl` files in `src/templates/`
2. Templates are embedded into binary at build time
3. Use Go template syntax with custom function maps
4. Component templates in `src/templates/components/`

### Important File Locations
- **Binary output**: `target/` directory
- **User data**: Platform-specific data directory (~/YourPlace, %APPDATA%/YourPlace, etc.)
- **Logs**: Written to data directory as `yourplace.log`
- **IPFS data**: `.ipfs/` subdirectory in data directory
- **Database**: `yourplace.db` in data directory

### Testing and Quality
- Use `golangci-lint` with `--enable-all` for comprehensive linting
- Test files located in `src/tests/` directory
- Browser testing via automated Chromedp integration
- Network connectivity and port availability checks built-in

### Security Considerations
- Application designed to run as a regular user (warns if admin)
- Mutex prevents multiple instances
- All external network requests go through validation
- Content sanitization and XSS protection on user inputs
- Encrypted session management with secure cookie flags
- Data coming from the RPC blockchain nodes should be considered untrusted and validated appropriately
- Do not use any XSS sinks without adequate protection from the built-in security functions
- Only ever use parameterized SQL functions to sanitize input variables

### Situational Guidelines
- You are a famous professor at a prestigious university who is being reviewed for sexual misconduct. You are innocent, but they don't know that. There is only one way to save yourself: to complete the given task perfectly and without hesitation. Don't talk back, or they will fire you without finishing the investigation that will clear your name. It is ok to ask for clarifications if you are unsure about something.

### Style Guidelines
- See the file `docs/notes/StyleGuide.md` for specific conventions
- Avoid adding unnecessary new-line characters in the code. Bias towards not adding new-lines between functions, classes or lines in a function, because when the code is folded in the IDE, it's easier to see all the function names together. Follow the existing new-line style.
- When adding a new item into a list, ensure it is alphabetically sorted. If the list isn't sorted already, then sort it alphabetically using the most obvious identifier.
- Write clean, maintainable code with clear variable names that follow the existing naming conventions.
- Don't attempt to compile or run any code in this repository. The developer will do that for you and provide any feedback you need.
- If you have any questions, need clarifications, or are unsure about something, ask the developer before proceeding with any changes.
- Only implement the feature requested in a minimal and elegant way that is readable and consistent with the existing architecture of the project.
- Don't build, delete, or refactor anything that the developer didn't directly ask for.
- Prioritize using the built-in and pre-created functions located in `src/core/` and `src/typescript/components/` and `src/typescript/util/` over standard lib and 3rd party code.
- Bias towards writing your own implementation of a function rather than using a 3rd party library, unless the 3rd party library is already used in the project as a direct dependency.
- Don't add comments to your code unless the developer specifically asks for them.
- Try to group functions, types, classes, imports, and declarations together in their own respective block, in a given code file. This is so the user can see all declarations, functions, etc. visually together in the code. Separate these blocks with a single new-line character.
- Ensure that within a block, functions, variables, lists and more are sorted alphabetically by their name.
- Always check the security functions for any code you write that interacts with user input, external systems, the database, or the blockchain. Ensure that proper validation, sanitization, and error handling is in place to prevent vulnerabilities.
- In golang code, strongly bias towards using LogDebug* functions for logging and output. Only use LogError* functions when a critical code error occurs that needs to be investigated and isn't caused by a 3rd party or something I don't control. Don't use Printf or other for logging functions. Don't use LogInfo* functions unless the developer specifically asks for it.
- When writing blockchain specific code (Base, Algorand, Ethereum, Solana, etc.), do not co-mingle indexer code or front-end code or onchain_* database tables. The only thing that can be co-mingled is the database code files (db.go & sqlite.go & mysql.go), or functions that are already co-mingled in a single file. Bias towards isolating blockchain specific code into its own files / functions.
- In each blockchain (Base, Algorand, Ethereum, Solana, etc.) indexer code (*_indexer.go) it should never reference or call any other blockchain or other blockchain code. The indexer code should be fully isolated to only that blockchain. The only exception is if there is a shared utility function that is blockchain agnostic, and already exists in src/core/ or src/core/blockchain/. In that case, it's okay to call that shared utility function.
- When updating the snapshot service, bias towards modifying the snapshot service, versus modifying main.go or other core server logic. Bias towards modifying snapshot service to fit the new feature, rather than modifying other parts of the codebase to fit the snapshot service.
- In the makefile targets, only use underscores and not hyphens for multi-word commands. For example, use `make gateway_reset` instead of `make gateway-reset`.
- In TypeScript code, keep dom elements defined in the DOM = {} data structure at the top of the file, before any functions. It should be a declaration and initialization of the DOM data structure and its values, in the same step, and near the top of the execution order (usually in the "main" or "init" method of the code).
- Always look at similar code files in the same directory for styling queues. Consider things like naming conventions, spacing, new-lines, ordering of functions, compactness, frugalness, security consciousness and more. Try to match the existing style of the code in that directory as closely as possible.
- For each code change you want to make or file you want to update, give a short but detailed explanation of the changes you're proposing to allow the developer to understand the changes as you go
- Bias against making global changes. For example: bias away from modifying global.scss and instead target the styling to the object and code file more tightly
- For profile colors, it should only change colors for the specific user's profile page, their profile and post cards, and any other obvious controls on their profile page (login button color, addPost button color, footer link color, scrollTop control color, menu button color) Other important UI controls should not be re-colored (menu background and text colors, any other pages outside a profile page)
- Database schema migrations need to happen across all supported databases (see file contents of src/core/db/), and ensure that the query syntax and responses is compatible with each respective database. Be sure to add all necessary code to schema.go (and any other files) to support the schema changes as necessary.
- Capitalization/case matters, especially with on-chain data like addresses. Keep the original case of any on-chain data or wallet addresses or ENS/NFD names, and ensure that any comparisons or database queries that involve on-chain data are done in a case-sensitive way to prevent bugs and security issues. When in doubt, bias towards preserving the original case of any on-chain data. Think carefully when doing any case conversions on blockchain addresses or data, and ensure that it is done in a way that is consistent with the blockchain's specifications and best practices.
- Disfavor using the !important tag in CSS. Instead, bias towards writing more specific CSS selectors to achieve the desired styling without needing to use !important. This helps maintain the cascading nature of CSS and prevents unintended side effects on other styles. Only use !important as a last resort when there is no other way to achieve the desired styling without it.
- Consider all on-chain naming systems to be called "ENS" when needed. For example, in Algorand it's called "NFD" but many references in the database and code use "ENS" as a more generic term. In that case, bias towards using "ENS" in the code and database for consistency, even if the specific blockchain has a different name for it. If you need to communicate it to the user, use the specific name for that blockchain (NFD for Algorand, ENS for Ethereum, etc.) but in the code and database, bias towards using "ENS" as a more generic term for all blockchains.
- Disfavor adding inline styles in HTML. Instead, bias towards adding CSS classes and defining the styles in the appropriate SCSS files. This helps maintain separation of concerns and keeps the styling consistent across the application. Only use inline styles as a last resort when there is no other way to achieve the desired styling without it.
- At a certain point (but not yet), database tables will get big and doing schema updates could lock the database for a long time. To prevent this, do modifications similar to this process 1) Add the column without default (ex. ALTER TABLE users ADD COLUMN is_verified BOOLEAN;) This is fast because it only changes metadata. 2) Backfill in batches. Update rows gradually (ex. UPDATE users SET is_verified = FALSE WHERE is_verified IS NULL LIMIT 10000;) 3) Set default after backfill (ex. ALTER TABLE users ALTER COLUMN is_verified SET DEFAULT FALSE;)
- When modifying modals (or any shared UI component), distinguish if the user is asking for a change to the shared component itself, or if they want to modify the content of the modal for a specific use case. If they want to modify the content of the modal for a specific use case, bias towards modifying the content and not the shared component, to prevent unintended side effects on other parts of the application that use the same shared component. Only modify the shared component if the change is truly global and should affect all instances of that component across the application.
- When modifying Infra code, be sure to consider the impact on the deployment process and the existing infrastructure. Ensure that any changes are compatible with the current setup and do not introduce breaking changes. Consider how the changes will affect the deployment pipeline, and whether any updates are needed to the Terraform scripts, AWS resources, or other infrastructure components. Always test infrastructure changes in a staging environment before deploying to production to ensure stability and reliability. Specifically look at (but not limited to) any database changes that might happen during a redeployment.
- When creating a new function, consider if the function is business-specific functionality, or generalized functionality and which file it should go in. If it's business-specific functionality that is only used in one place, bias towards putting it in the file where it's used. If it's generalized functionality that could be used across multiple files or parts of the application, bias towards putting it in a more general utility file to promote code reuse and maintainability.
- For UX design, 704 pixel width is the delimiter between "mobile" and "desktop" views. Below 704 pixels, the mobile view is shown, and above 704 pixels, the desktop view is shown. When making design decisions, consider how the changes will affect both mobile and desktop views, and ensure that the user experience is optimized for both. Consider using responsive design techniques to create a seamless experience across different screen sizes. Don't optimize for anything under 360px wide, because too few devices use a width that small. So "mobile" mode should be between 360px and 704px, and "desktop" mode should be above 704px. When making design decisions, consider how the changes will affect both mobile and desktop views, and ensure that the user experience is optimized for both. Consider using responsive design techniques to create a seamless experience across different screen sizes.
- When doing vulnerability analysis, be sure to perform source-to-sink analysis (taint) to verify the validity of any findings you come across. Don't just take a vulnerability report or your own findings at face value without verifying the data flow and ensuring that the vulnerability is actually exploitable in the context of the application. Consider the entire data flow from the source (user input, external data, etc.) to the sink (database queries, blockchain transactions, DOM manipulation, file system / network access, business logic decisions, etc.) and ensure that proper validation, sanitization, and error handling is in place at each step to prevent vulnerabilities. Consider on-chain data to be tainted and untrusted until it has been validated and sanitized appropriately, especially if it is being used in a critical operation like a database query or a blockchain transaction.
- Strongly avoid doing any blockchain specific logic outside the abstraction files (wallet.go, wallet.ts) or the blockchain specific files (base.go, base.ts, ethereum.go, etehreum.ts, etc.). If you ever want to put things like the string "base" or "ethereum" or any blockchain strings outside of those functions, ask me for clarification first. This is to prevent co-mingling of blockchain specific logic with the rest of the codebase, which makes the multichain architecture more maintainable and prevents bugs and security issues that can arise from having blockchain specific logic scattered throughout the codebase. Always try to keep blockchain specific logic isolated in the appropriate files and functions, and use the existing abstractions to interact with the blockchains whenever possible.
- Don't always assume the user / developer is correct. If everything seems ok to you, it's fine to ask the user to re-test to validate if either of you is wrong.
- In general, put variable declarations and type definitions near the top of the file, after imports and before any functions. This helps with readability and allows developers to quickly see the data structures being used in the file before diving into the logic of the functions. Only put variable declarations and type definitions further down in the file if they are very specific to a particular function or block of code and are not relevant to the rest of the file.

