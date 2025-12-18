# Post Card Social Interaction Features - Implementation Plan

## Overview
This plan outlines the implementation of rich social interaction features for the YourPlace platform, including comments, likes, dislikes, emoji reactions, and reposts. All interactions are stored on-chain and indexed into the local database for efficient querying.

## Table of Contents
1. [Database Schema Changes](#1-database-schema-changes)
2. [Blockchain JSON API Extensions](#2-blockchain-json-api-extensions)
3. [Backend Indexer Updates](#3-backend-indexer-updates)
4. [Backend Routes & API](#4-backend-routes--api)
5. [Frontend TypeScript Changes](#5-frontend-typescript-changes)
6. [UI/UX Components](#6-uiux-components)
7. [Styling (SCSS)](#7-styling-scss)
8. [Documentation Updates](#8-documentation-updates)
9. [File Change Summary](#9-file-change-summary)

---

## 1. Database Schema Changes

### New Tables Required

#### 1.1 Comments Table (Base blockchain)
```sql
CREATE TABLE IF NOT EXISTS onchain_comment (
    txHash TEXT,
    blockchain TEXT,
    fromAddress TEXT DEFAULT '',
    parentTxHash TEXT DEFAULT '',
    parentType TEXT DEFAULT 'post',  -- 'post' or 'comment'
    amount REAL DEFAULT 0,
    timestamp INTEGER DEFAULT 0,
    data TEXT DEFAULT '',
    PRIMARY KEY(txHash, blockchain)
)
```

#### 1.2 Comments Table (Algorand blockchain)
```sql
CREATE TABLE IF NOT EXISTS onchain_algorand_comment (
    txHash TEXT,
    blockchain TEXT,
    fromAddress TEXT DEFAULT '',
    parentTxHash TEXT DEFAULT '',
    parentType TEXT DEFAULT 'post',
    amount REAL DEFAULT 0,
    timestamp INTEGER DEFAULT 0,
    data TEXT DEFAULT '',
    PRIMARY KEY(txHash, blockchain)
)
```

#### 1.3 Reactions Table (Base blockchain)
```sql
CREATE TABLE IF NOT EXISTS onchain_reaction (
    txHash TEXT,
    blockchain TEXT,
    fromAddress TEXT DEFAULT '',
    targetTxHash TEXT DEFAULT '',
    targetType TEXT DEFAULT 'post',  -- 'post' or 'comment'
    reactionType TEXT DEFAULT '',     -- 'like', 'dislike', or emoji unicode
    timestamp INTEGER DEFAULT 0,
    PRIMARY KEY(txHash, blockchain)
)
```

#### 1.4 Reactions Table (Algorand blockchain)
```sql
CREATE TABLE IF NOT EXISTS onchain_algorand_reaction (
    txHash TEXT,
    blockchain TEXT,
    fromAddress TEXT DEFAULT '',
    targetTxHash TEXT DEFAULT '',
    targetType TEXT DEFAULT 'post',
    reactionType TEXT DEFAULT '',
    timestamp INTEGER DEFAULT 0,
    PRIMARY KEY(txHash, blockchain)
)
```

### Schema Migration
- Update `src/core/db/schema.go` - Increment `SchemaVersion` to 2
- Add migration function `migrateV2()` to create new tables

### Database Functions Required
Files: `src/core/db/sqlite.go`, `src/core/db/database.go`

```go
// Comment functions
OnchainC(txHash, blockchain, fromAddress, parentTxHash, parentType, amount, timestamp, data string)
OnchainAlgoC(txHash, blockchain, fromAddress, parentTxHash, parentType, amount, timestamp, data string)
GetComments(parentTxHash, blockchain string, limit, offset int) []Comment
GetCommentCount(targetTxHash, blockchain string) int
GetCommentThread(rootTxHash, blockchain string) []CommentThread

// Reaction functions
OnchainR(txHash, blockchain, fromAddress, targetTxHash, targetType, reactionType string, timestamp uint64)
OnchainAlgoR(txHash, blockchain, fromAddress, targetTxHash, targetType, reactionType string, timestamp uint64)
GetReactionCounts(targetTxHash, blockchain string) ReactionCounts
GetUserReaction(targetTxHash, blockchain, fromAddress string) string
GetEmojiReactions(targetTxHash, blockchain string) map[string]int
```

---

## 2. Blockchain JSON API Extensions

### New Action Codes
File: `src/typescript/services/yourplace.ts`

| Action | Code | Description |
|--------|------|-------------|
| Comment | `c` | Post a comment on a post |
| Comment with attachment | `ca` | Comment with file attachments |
| Like | `rl` | Like a post or comment |
| Dislike | `rdl` | Dislike a post or comment |
| Emoji Reaction | `re` | React with emoji |

### JSON Payload Structures

#### Comment (action code: `c`)
```json
yp/1/c:{"t":"parentTxHash","p":"comment text"}
```
- `t` = target transaction hash (post or parent comment)
- `p` = comment payload/text

#### Comment with Attachment (action code: `ca`)
```json
yp/1/ca:{"t":"parentTxHash","p":"comment text","a":[["ipfs://CID","mime/type",size,"filename"]]}
```

#### Like (action code: `rl`)
```json
yp/1/rl:{"t":"targetTxHash","y":"post"}
```
- `t` = target transaction hash
- `y` = target type ('post' or 'comment')

#### Dislike (action code: `rdl`)
```json
yp/1/rdl:{"t":"targetTxHash","y":"post"}
```

#### Emoji Reaction (action code: `re`)
```json
yp/1/re:{"t":"targetTxHash","y":"post","e":"😀"}
```
- `e` = emoji unicode character(s)

---

## 3. Backend Indexer Updates

### Files to Modify
- `src/core/db/blockchain/yourplace.go` - Add transaction parsing for new action codes
- `src/core/db/blockchain/base_indexer.go` - No changes needed (uses yourplace.go)
- `src/core/db/blockchain/algorand_indexer.go` - Add Algorand-specific parsing

### Transaction Parsing Logic

#### In `yourplace.go` - `tokenizeYourPlaceTransaction()`

Add new case handlers in the switch statement:

```go
case 'c': // Comment Actions
    switch actionPostfix {
    case "":  // Plain comment
        handleCommentTransaction(payloadObject, txHash, blockchain, fromAddress, amountInt, timestamp, blockNumber)
    case "a": // Comment with attachments
        handleCommentTransactionAttachment(payloadObject, txHash, blockchain, fromAddress, amountInt, timestamp, blockNumber)
    }

case 'r': // Reaction Actions (update existing stub)
    switch actionPostfix {
    case "l":  // Like
        handleLikeTransaction(payloadObject, txHash, blockchain, fromAddress, timestamp)
    case "dl": // Dislike
        handleDislikeTransaction(payloadObject, txHash, blockchain, fromAddress, timestamp)
    case "e":  // Emoji reaction
        handleEmojiReactionTransaction(payloadObject, txHash, blockchain, fromAddress, timestamp)
    }
```

### Reaction Deduplication Logic
- Only count the most recent reaction per account per target
- When inserting a new reaction, check if one exists from the same address
- Use `INSERT OR REPLACE` or `UPSERT` semantics

---

## 4. Backend Routes & API

### New Routes
File: `src/routes/post.go` (expand existing) + new files

#### 4.1 Single Post Page Route
```go
// GET /post/:blockchain/:hash - Renders dedicated post page with comments
router.GET("/post/:blockchain/:hash", PostPageHandler)
```

#### 4.2 Comments API Routes
File: `src/routes/comment.go` (new file)
```go
// GET /comments/:blockchain/:txHash - Get comments for a post/comment
router.GET("/comments/:blockchain/:txHash", GetCommentsHandler)

// GET /comments/:blockchain/:txHash/count - Get comment count
router.GET("/comments/:blockchain/:txHash/count", GetCommentCountHandler)

// GET /comments/:blockchain/:txHash/thread - Get full comment thread
router.GET("/comments/:blockchain/:txHash/thread", GetCommentThreadHandler)
```

#### 4.3 Reactions API Routes
File: `src/routes/reaction.go` (new file)
```go
// GET /reactions/:blockchain/:txHash - Get reaction counts for a post/comment
router.GET("/reactions/:blockchain/:txHash", GetReactionsHandler)

// GET /reactions/:blockchain/:txHash/user/:address - Get user's reaction
router.GET("/reactions/:blockchain/:txHash/user/:address", GetUserReactionHandler)

// GET /reactions/:blockchain/:txHash/emoji - Get emoji reaction breakdown
router.GET("/reactions/:blockchain/:txHash/emoji", GetEmojiReactionsHandler)
```

### Response Structures

#### Comments Response
```json
{
    "comments": [
        {
            "txHash": "0x...",
            "blockchain": "base",
            "fromAddress": "0x...",
            "parentTxHash": "0x...",
            "parentType": "post",
            "timestamp": 1234567890,
            "data": "comment text",
            "author": "username",
            "avatarSrc": "ipfs://...",
            "likeCount": 5,
            "dislikeCount": 1,
            "replyCount": 3
        }
    ],
    "total": 10
}
```

#### Reactions Response
```json
{
    "likes": 42,
    "dislikes": 3,
    "emoji": {
        "😀": 5,
        "❤️": 12,
        "🔥": 8
    },
    "userReaction": "like"  // null if user hasn't reacted
}
```

### Template for Post Page
File: `src/templates/pages/post.tmpl` (new file)

---

## 5. Frontend TypeScript Changes

### 5.1 YourPlace Protocol Extensions
File: `src/typescript/services/yourplace.ts`

Add new functions:
```typescript
comment: function(parentTxHash: string, text: string): string
commentAttach: function(parentTxHash: string, text: string, attach: string[][]): string
like: function(targetTxHash: string, targetType: string): string
dislike: function(targetTxHash: string, targetType: string): string
emojiReact: function(targetTxHash: string, targetType: string, emoji: string): string
```

### 5.2 Wallet Functions
File: `src/typescript/util/blockchain/wallet.ts`

Add new exported functions:
```typescript
WalletSubmitComment(parentTxHash: string, payload: string): Promise<boolean>
WalletSubmitCommentAttach(parentTxHash: string, payload: string, attach: string[][]): Promise<boolean>
WalletSubmitLike(targetTxHash: string, targetType: string): Promise<boolean>
WalletSubmitDislike(targetTxHash: string, targetType: string): Promise<boolean>
WalletSubmitEmojiReaction(targetTxHash: string, targetType: string, emoji: string): Promise<boolean>
```

### 5.3 Base Blockchain Functions
File: `src/typescript/util/blockchain/base.ts`

Add corresponding base-specific functions:
```typescript
baseSubmitComment(parentTxHash: string, payload: string): Promise<string>
baseSubmitCommentAttach(parentTxHash: string, payload: string, attach: string[][]): Promise<string>
baseSubmitLike(targetTxHash: string, targetType: string): Promise<string>
baseSubmitDislike(targetTxHash: string, targetType: string): Promise<string>
baseSubmitEmojiReaction(targetTxHash: string, targetType: string, emoji: string): Promise<string>
```

### 5.4 Local Wallet Functions
File: `src/typescript/util/blockchain/localWallet.ts`

Mirror the base functions for local wallet support.

### 5.5 New Component: Post Controls Bar
File: `src/typescript/components/postControls.ts` (new file)

```typescript
interface PostControlsOptions {
    txHash: string;
    blockchain: string;
    targetType: 'post' | 'comment';
    initialLikes?: number;
    initialDislikes?: number;
    initialComments?: number;
    userReaction?: string | null;
}

export function CreatePostControlsBar(options: PostControlsOptions): HTMLDivElement
export function UpdateReactionCounts(controlsBar: HTMLDivElement, counts: ReactionCounts): void
```

### 5.6 New Component: Comment Thread
File: `src/typescript/components/commentThread.ts` (new file)

```typescript
interface CommentThreadOptions {
    parentTxHash: string;
    blockchain: string;
    maxDepth?: number;  // Default 4
}

export function CreateCommentThread(options: CommentThreadOptions): HTMLDivElement
export function ExpandCommentThread(threadDiv: HTMLDivElement): void
export function CollapseCommentThread(threadDiv: HTMLDivElement): void
```

### 5.7 New Component: Add Comment
File: `src/typescript/components/addComment.ts` (new file)

Reuse TinyMCE initialization pattern from `addPost.ts`:
```typescript
export function CreateAddCommentUI(parentTxHash: string, blockchain: string): HTMLDivElement
export function ShowAddCommentModal(parentTxHash: string, blockchain: string): void
```

### 5.8 New Component: Emoji Picker
File: `src/typescript/components/emojiPicker.ts` (new file)

```typescript
export function CreateEmojiPicker(onSelect: (emoji: string) => void): HTMLDivElement
export function ShowReactionsPopup(targetElement: HTMLElement, txHash: string, blockchain: string): void
```

### 5.9 New Component: Post Preview Card (for reposts)
File: `src/typescript/components/postPreviewCard.ts` (new file)

```typescript
export function CreatePostPreviewCard(postUrl: string): Promise<HTMLDivElement>
export function DetectPostUrl(text: string): string | null
```

### 5.10 API Client Functions
File: `src/typescript/util/network.ts` (extend existing)

Add functions for fetching reactions and comments:
```typescript
export function FetchReactions(blockchain: string, txHash: string): Promise<ReactionCounts>
export function FetchComments(blockchain: string, txHash: string, limit?: number, offset?: number): Promise<Comment[]>
export function FetchCommentThread(blockchain: string, txHash: string): Promise<CommentThread[]>
```

### 5.11 DOM Factory Updates
File: `src/typescript/util/domFactory.ts`

Update `CreatePostCard()` to:
1. Add post controls bar to `reactionDiv`
2. Handle post preview cards when `/post/` URLs detected in content

---

## 6. UI/UX Components

### 6.1 Post Controls Bar Layout
```
┌─────────────────────────────────────────────────────────────────┐
│  💬 12    👍 42    👎 3    😀 25    🔄 5                         │
│  Comment  Like    Dislike  React   Repost                       │
└─────────────────────────────────────────────────────────────────┘
```

Each control:
- Bootstrap icon in grey (`bi-chat`, `bi-hand-thumbs-up`, `bi-hand-thumbs-down`, `bi-emoji-smile`, `bi-arrow-repeat`)
- Count displayed next to icon
- Hover tooltip showing action name
- Hover color change for feedback
- Active state for user's selected reaction

### 6.2 Comment Input (TinyMCE Slide-down)
When user clicks comment:
1. Smooth CSS animation slides down TinyMCE editor below controls
2. Same features as post editor (formatting, emoticons, attachments)
3. "Post Comment" and "Cancel" buttons
4. Animation: `max-height` transition with `overflow: hidden`

### 6.3 Comment Thread Structure
```
Post
├── Comment 1 (depth 0)
│   ├── Reply 1.1 (depth 1)
│   │   ├── Reply 1.1.1 (depth 2)
│   │   │   └── Reply 1.1.1.1 (depth 3) [max indent]
│   │   │       └── Reply 1.1.1.1.1 (collapsed, starts new chain)
│   └── Reply 1.2 (depth 1)
└── Comment 2 (depth 0)
```

Features:
- Each comment indented with left border/padding
- Collapse/expand toggle icon (chevron)
- All comments start collapsed, expandable on click
- After depth 4, reset indentation and collapse parent
- Sort by like count (descending)
- "Show more replies" button for pagination

### 6.4 Reactions Popup
When user clicks react icon:
1. Popup appears showing existing emoji reactions with counts
2. User can click existing emoji to add their reaction
3. "Add reaction" button opens emoji picker
4. Click outside closes popup
5. User's selected emoji highlighted

### 6.5 Repost Flow
When user clicks repost:
1. Opens Add Post modal (existing)
2. Pre-fills with `/post/:blockchain/:hash` URL
3. User can add their own commentary
4. Post renders with embedded preview card

### 6.6 Post Preview Card (Embedded Repost)
```
┌─────────────────────────────────────────────────────────┐
│ ┌─────────────────────────────────────────────────────┐ │
│ │ [Avatar] Author Name                     [Date]     │ │
│ │ Original post text content...                       │ │
│ │ [Media preview if applicable]                       │ │
│ └─────────────────────────────────────────────────────┘ │
│ User's commentary on the repost...                      │
└─────────────────────────────────────────────────────────┘
```

---

## 7. Styling (SCSS)

### New File: `src/scss/components/postControls.scss`
```scss
.postControlsBar {
    display: flex;
    justify-content: flex-start;
    gap: 1.5em;
    padding: 0.5em 0;
    margin-left: 5em;  // Align with post text
}

.postControlItem {
    display: flex;
    align-items: center;
    gap: 0.3em;
    color: #6c757d;  // Grey
    cursor: pointer;
    transition: color 0.2s ease;

    &:hover {
        color: #primary;
    }

    &.active {
        color: #primary;
    }

    .count {
        font-size: 0.85em;
    }
}

.postControlItem.like.active { color: #28a745; }
.postControlItem.dislike.active { color: #dc3545; }
```

### New File: `src/scss/components/commentThread.scss`
```scss
.commentThread {
    margin-left: 5em;
    border-left: 2px solid #secondary;
}

.commentItem {
    padding: 0.75em;
    margin-left: var(--indent-level, 0);

    &[data-depth="1"] { --indent-level: 1.5em; }
    &[data-depth="2"] { --indent-level: 3em; }
    &[data-depth="3"] { --indent-level: 4.5em; }
    &[data-depth="4"] { --indent-level: 4.5em; }  // Max indent
}

.commentToggle {
    cursor: pointer;
    user-select: none;

    .bi-chevron-down { transition: transform 0.2s; }
    &.collapsed .bi-chevron-down { transform: rotate(-90deg); }
}

.commentReplies {
    max-height: 1000px;
    overflow: hidden;
    transition: max-height 0.3s ease;

    &.collapsed {
        max-height: 0;
    }
}
```

### New File: `src/scss/components/addComment.scss`
```scss
.addCommentContainer {
    max-height: 0;
    overflow: hidden;
    transition: max-height 0.3s ease-out;
    margin-left: 5em;

    &.expanded {
        max-height: 500px;
    }
}

.addCommentActions {
    display: flex;
    gap: 0.5em;
    justify-content: flex-end;
    margin-top: 0.5em;
}
```

### New File: `src/scss/components/emojiPicker.scss`
```scss
.reactionsPopup {
    position: absolute;
    background: #primary;
    border: 1px solid #secondary;
    border-radius: 0.5em;
    padding: 0.75em;
    box-shadow: 0 4px 12px rgba(0,0,0,0.15);
    z-index: 1000;
}

.existingReactions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5em;
    margin-bottom: 0.5em;
}

.reactionChip {
    display: flex;
    align-items: center;
    gap: 0.25em;
    padding: 0.25em 0.5em;
    background: #secondary;
    border-radius: 1em;
    cursor: pointer;

    &:hover, &.selected {
        background: #tertiary;
    }

    .emoji { font-size: 1.2em; }
    .count { font-size: 0.8em; }
}

.emojiGrid {
    display: grid;
    grid-template-columns: repeat(8, 1fr);
    gap: 0.25em;
    max-height: 200px;
    overflow-y: auto;
}

.emojiButton {
    font-size: 1.5em;
    padding: 0.25em;
    background: transparent;
    border: none;
    cursor: pointer;
    border-radius: 0.25em;

    &:hover {
        background: #secondary;
    }
}
```

### New File: `src/scss/components/postPreviewCard.scss`
```scss
.postPreviewCard {
    border: 1px solid #secondary;
    border-radius: 0.5em;
    padding: 0.75em;
    margin: 0.5em 0;
    background: rgba(0,0,0,0.02);

    .previewHeader {
        display: flex;
        align-items: center;
        gap: 0.5em;
        margin-bottom: 0.5em;
    }

    .previewAvatar {
        width: 2em;
        height: 2em;
        border-radius: 0.3em;
    }

    .previewAuthor {
        font-weight: 500;
    }

    .previewDate {
        font-size: 0.8em;
        opacity: 0.7;
    }

    .previewText {
        font-size: 0.95em;
        max-height: 100px;
        overflow: hidden;
    }

    .previewMedia {
        max-height: 150px;
        overflow: hidden;
        border-radius: 0.3em;
        margin-top: 0.5em;

        img {
            width: 100%;
            object-fit: cover;
        }
    }
}
```

### Update: `src/scss/components/postCard.scss`
Add import and styling for controls bar integration.

---

## 8. Documentation Updates

### 8.1 BlockchainAPI.md
Add new action codes with 🚢 status after implementation:
- Comment actions (`c`, `ca`)
- Reaction actions (`rl`, `rdl`, `re`)

### 8.2 DatabaseSchema.md
Add new table documentation:
- `onchain_comment`
- `onchain_algorand_comment`
- `onchain_reaction`
- `onchain_algorand_reaction`

### 8.3 API Documentation
Document new HTTP endpoints for comments and reactions.

---

## 9. File Change Summary

### New Files
| Path | Description |
|------|-------------|
| `src/core/db/blockchain/comment.go` | Comment transaction handling |
| `src/core/db/blockchain/reaction.go` | Reaction transaction handling |
| `src/routes/comment.go` | Comment API routes |
| `src/routes/reaction.go` | Reaction API routes |
| `src/templates/pages/post.tmpl` | Single post page template |
| `src/typescript/components/addComment.ts` | Add comment UI component |
| `src/typescript/components/commentThread.ts` | Comment thread component |
| `src/typescript/components/emojiPicker.ts` | Emoji picker component |
| `src/typescript/components/postControls.ts` | Post controls bar component |
| `src/typescript/components/postPreviewCard.ts` | Repost preview card |
| `src/scss/components/addComment.scss` | Add comment styles |
| `src/scss/components/commentThread.scss` | Comment thread styles |
| `src/scss/components/emojiPicker.scss` | Emoji picker styles |
| `src/scss/components/postControls.scss` | Controls bar styles |
| `src/scss/components/postPreviewCard.scss` | Preview card styles |

### Modified Files
| Path | Changes |
|------|---------|
| `src/core/db/schema.go` | Add migration for new tables |
| `src/core/db/sqlite.go` | Add new tables to createTables(), add CRUD functions |
| `src/core/db/database.go` | Add interface methods for new functions |
| `src/core/db/blockchain/yourplace.go` | Add comment and reaction parsing |
| `src/routes/post.go` | Add single post page route |
| `src/typescript/services/yourplace.ts` | Add protocol functions |
| `src/typescript/util/blockchain/wallet.ts` | Add wallet submit functions |
| `src/typescript/util/blockchain/base.ts` | Add base blockchain functions |
| `src/typescript/util/blockchain/localWallet.ts` | Add local wallet functions |
| `src/typescript/util/domFactory.ts` | Integrate controls bar, preview cards |
| `src/typescript/util/network.ts` | Add API fetch functions |
| `src/scss/components/postCard.scss` | Import and integrate new styles |
| `docs/BlockchainAPI.md` | Document new action codes |
| `docs/DatabaseSchema.md` | Document new tables |

---

## Implementation Order

### Phase 1: Database & Backend Foundation
1. Database schema changes (sqlite.go, schema.go)
2. Database CRUD functions
3. YourPlace protocol parsing (yourplace.go)
4. Backend API routes

### Phase 2: Frontend Protocol & Wallet
1. YourPlace.ts protocol extensions
2. Wallet functions (wallet.ts, base.ts, localWallet.ts)
3. Network API functions

### Phase 3: UI Components
1. Post controls bar component
2. Comment thread component
3. Add comment component (TinyMCE integration)
4. Emoji picker and reactions popup
5. Post preview card

### Phase 4: Integration & Polish
1. Integrate controls into CreatePostCard()
2. Single post page template
3. SCSS styling
4. Documentation updates

### Phase 5: Testing
1. Test on-chain transaction submission
2. Test indexer parsing
3. Test comment threading
4. Test reaction counting and deduplication
5. Cross-browser testing

---

## Notes

### Comment Thread Depth Handling
- Comments sorted by like count descending within each level
- Maximum visual indent depth: 4 levels
- After depth 4, replies continue but without additional indentation
- Parent collapse resets the visual hierarchy for deep threads

### Reaction Deduplication
- Database stores all reactions with timestamps
- Queries return most recent reaction per user per target
- Count queries aggregate only most recent per user
- User can change reaction by submitting new one (replaces old)

### Repost Detection
- Regex pattern: `/\/post\/(base|algorand)\/0x[a-fA-F0-9]+/`
- When detected in post content, fetch and render preview card
- Preview card is clickable, links to full post page

### Mobile Responsiveness
- Controls bar wraps on small screens
- Comment indentation reduced on mobile
- Emoji picker adapts to screen width
- Touch-friendly tap targets (min 44px)
