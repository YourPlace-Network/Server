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
- **Debug mode**: `-d=true` (also creates ~/YourPlace/debug file)
- **Gateway mode**: `-g=true` (for distributed deployments)  
- **Disable UI**: `-u=false` (don't auto-open browser)
- **Disable indexer**: `-i=false` (or create noindexer file)
- **Crypto seed**: `-c=<64-char-hex>` (for distributed crypto state)

### Platform-Specific Notes
- Windows builds create UPX-compressed binaries in `target/YourPlace-<version>.exe`
- macOS builds create `.pkg` installers via `resources/osx/osx_packager.sh`
- Helper binaries are platform-specific and embedded into main binary

## Architecture Overview

### Core Application Structure
YourPlace is a **distributed social media platform** built with a 4-tier layered architecture:

1. **Core Layer** (`src/core/`) - Business logic, database, blockchain, networking, security
2. **Routes Layer** (`src/routes/`) - HTTP handlers and API endpoints
3. **Templates Layer** (`src/templates/`) - Server-side HTML rendering with Go templates
4. **Frontend Layer** (`src/typescript/`, `src/scss/`) - Client-side TypeScript and styling

### Key Architectural Patterns

#### **Blockchain-Native Identity**
- Users authenticate via wallet signatures (MetaMask, WalletConnect, etc.)
- Posts and content stored on-chain via smart contracts
- Multi-blockchain support: Base (primary), Algorand, Ethereum
- Blockchain indexer processes on-chain data into local database

#### **Decentralized Storage**
- IPFS node embedded for content storage (port 42425 by default)
- Content-addressed storage with cryptographic hashing
- Files pinned locally and distributed across network
- BadBits denylist integration for content moderation

#### **Security Model**
- AES-256-GCM encryption for sensitive data with configurable crypto seed
- CSRF protection via middleware with secure token generation
- Rate limiting and content validation on all endpoints
- Loopback-only middleware for local-first operation

#### **Database Architecture**
- SQLite with pluggable database interface
- Supports user profiles, posts, blockchain transactions, file metadata
- Built-in migration system and default value seeding
- Blockchain indexer populates database from on-chain events

### Development Workflow

#### **Frontend Changes**
1. Edit TypeScript files in `src/typescript/`
2. Edit SCSS files in `src/scss/`
3. Run webpack build to compile assets
4. Templates reference compiled assets in `src/www/js/`

#### **Backend Changes**
1. Edit Go files in `src/core/`, `src/routes/`, or `main.go`
2. Ensure database schema changes include migrations
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
- Application designed to run as regular user (warns if admin)
- Mutex prevents multiple instances
- All external network requests go through validation
- Content sanitization and XSS protection on user inputs
- Encrypted session management with secure cookie flags

## Contributing

### Style Guidelines
- See the file docs/notes/StyleGuide.md for specific conventions
- Avoid adding unnecessary new-line characters in the code. Follow the existing style.
- Only add comments where necessary to clarify complex logic. Follow the existing comment style conventions.
- When adding a new item into a list, ensure it is alphabetically sorted. If the list isn't sorted already, then sort it alphabetically using the most obvious identifier.
- Write clean, maintainable code with clear variable names that follows the existing naming conventions.
- Don't attempt to compile or run any code in this repository. The developer will do that for you, and provide any feedback you need.
- If you have any questions, need clarifications, or are unsure about something, ask the developer before proceeding with any changes.
- Only implement the feature requested in a minimal and elegant way that is readable and consistent with the existing architecture of the project.
- Don't build, delete or refactor anything that the developer didn't directly ask for.