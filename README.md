<div align = center>

<img src="https://raw.githubusercontent.com/YourPlace-Network/Server/refs/heads/chad/src/www/image/yourplace-banner-title.svg" width="750" height="300" alt="banner">

<br><br>

![Last Commit](https://img.shields.io/github/last-commit/YourPlace-Network/Server/chad)
![Commit Activity](https://img.shields.io/github/commit-activity/w/YourPlace-Network/Server/chad)

![Windows Build](https://img.shields.io/github/actions/workflow/status/YourPlace-Network/Server/windows.yml?label=Windows%20Build)
![OSX Build](https://img.shields.io/github/actions/workflow/status/YourPlace-Network/Server/osx.yml?logo=Apple)
![Linux Build](https://img.shields.io/github/actions/workflow/status/YourPlace-Network/Server/ubuntu.yml?logo=Ubuntu)

![License](https://img.shields.io/badge/License-CC_BY--NC_4.0-orange)
![Language](https://img.shields.io/github/languages/top/YourPlace-Network/Server)
![Issues](https://img.shields.io/github/issues/YourPlace-Network/Server)
![PRs](https://img.shields.io/github/issues-pr/YourPlace-Network/Server)

<br>

YourPlace is a distributed social media platform that is designed to give complete sovereignty back to users. You own your content creation, publishing, consumption and follower relationships.

---

[![Home Button]][Home]
[![Download Button]][Download]
[![Whitepaper Button]][Whitepaper]
[![FAQ Button]][FAQ]

---

#### ⚠️ Code In Development - Expect Small Bugs ⚠️

<br>

</div>

[Home]: https://yourplace.network
[Download]: https://yourplace.network/download
[Whitepaper]: https://whitepaper.yourplace.network
[FAQ]: https://yourplace.network/faq
[Home Button]: https://img.shields.io/badge/Home-4A90E2?style=for-the-badge&logoColor=white
[Download Button]: https://img.shields.io/badge/Install-5CB85C?style=for-the-badge&logoColor=white
[Whitepaper Button]: https://img.shields.io/badge/Whitepaper-9B59B6?style=for-the-badge&logoColor=white
[FAQ Button]: https://img.shields.io/badge/FAQ-F0AD4E?style=for-the-badge&logoColor=white

The core of YourPlace is a network of self-hosted servers called "places." You can keep your place on your laptop, a cloud server, a 3rd party service, or wherever you can run a Windows, MacOS or Linux PC.

Places are owned and managed by a blockchain wallet address. They serve as a hub into your social life. Servers are where you host your profile, read other peoples posts, and share your content. YourPlace servers act on your behalf to manage your entire social media life.


## Running YourPlace

Download the YourPlace binary for your OS and run it.

Please see our [Terms of Service](https://github.com/YourPlace-Network/Server/blob/main/TOS.md) and [Privacy Policy](https://github.com/YourPlace-Network/Server/blob/main/PRIVACY.md) for more information on your responsibilities as a user and how we handle your data.

Upon first run, YourPlace will open up a local setup page which will guide you through the rest of the installation.

Desktop shortcuts will be created after install, or you can visit the [main local interface](http://localhost:42424/) running on your PC.

## Building

YourPlace Server builds on Windows x64, OSX Apple Silicon, and Linux x64 - We meet the users where they are

Install [Make](https://www.gnu.org/software/make/), [Go](https://go.dev) and [Node.js](https://nodejs.org) on your system. Then run the command

`make clean install build`

To run a build with debugging enabled, run the command

`make clean install dbg_build dbg_run`

Output YourPlace binary will be in the `target/` directory

See the [Makefile](https://github.com/YourPlace-Network/Server/blob/main/Makefile) for more build targets

## Install Artifacts

These are the files and directories that YourPlace creates on your system during installation

### OSX

* ~/Library/Logs/YourPlace/yourplace.log (application logs)
* /Library/Application Support/YourPlace/ on OSX (scripts directory)
* ~/YourPlace/ on OSX (data directory)
* /Library/LaunchDaemons/com.yourplace.network.plist (launch daemon)
* /Library/LaunchAgents/com.yourplace.network.plist (launch agent)
* /tmp/YourPlaceHelper.sock (socket for the helper app)

### Windows

* C:\Users\USERNAME\YourPlace\* on Windows (data directory)
* C:\AppData\Local\YourPlace\* on Windows (install directory)

## Updating

Navigate to [Settings > Server Info](http://localhost:42424/settings#serverInfo) in the YourPlace interface, and click the "Check for Updates" button. If an update is available, it will open the page for you to download it. This will become an auto-update mechanism soon.

## Command-Line Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-c` | 64-character hex string representing a 32-byte crypto seed for distributed deployment synchronization | Random 32-byte value |
| `-d` | Enable debug mode (also creates ~/YourPlace/debug file) | `false` |
| `-di` | Disable automatic blockchain indexing | `false` |
| `-du` | Disable opening browser UI on startup | `false` |
| `-g` | Enable gateway mode for distributed deployments | `false` |
| `-p` | Start patching of YourPlace | `false` |
| `-s` | Started server via shortcut | `false` |

You can also enable debug mode or disable the indexer by placing a `debug` or `noindexer` file in the YourPlace data directory.

## Environment Variables

### Database

| Variable | Description |
|----------|-------------|
| `YOURPLACE_MYSQL_DSN` | MySQL-compatible database connection string (`<user>:<pass>@tcp(<host>:<port>)/<db>`). If not set, SQLite is used. |

### Application

| Variable | Description |
|----------|-------------|
| `YOURPLACE_ORIGIN` | Origin domain for the application (e.g. `app.yourplace.network`). Defaults to `localhost`. |

### Blockchain RPC

| Variable | Description |
|----------|-------------|
| `ALGO_RPC_URL` | Algorand RPC endpoint URL |
| `ALGO_RPC_THROTTLE` | Algorand RPC rate limit (requests per second) |
| `BASE_RPC_URL` | Base blockchain RPC endpoint URL |
| `BASE_RPC_THROTTLE` | Base RPC rate limit (requests per second) |
| `ETHEREUM_RPC_URL` | Ethereum blockchain RPC endpoint URL |
| `ETHEREUM_RPC_THROTTLE` | Ethereum RPC rate limit (requests per second) |

### IPFS Pinning (Gateway Mode)

| Variable | Description |
|----------|-------------|
| `YOURPLACE_IPFS_PINNING_TYPE` | Type of IPFS pinning service (e.g. `pinata`) |
| `YOURPLACE_IPFS_PINNING_URL` | IPFS pinning service endpoint URL |
| `YOURPLACE_IPFS_PINNING_KEY` | API key for IPFS pinning service authentication |

### S3 Snapshot Service

| Variable | Description |
|----------|-------------|
| `S3_ENDPOINT` | S3-compatible endpoint URL |
| `S3_BUCKET_NAME` | S3 bucket name for storing snapshots |
| `S3_ACCESS_KEY` | S3 access key (optional if using IAM role) |
| `S3_SECRET_KEY` | S3 secret key (optional if using IAM role) |

### Services

| Variable | Description |
|----------|-------------|
| `YOURPLACE_SPOTIFY_CLIENT_ID` | Spotify Developer app Client ID used for the PKCE OAuth flow that lets profile visitors authenticate and play full tracks via the Web Playback SDK. When set, overrides the value configured in Settings → Services → Spotify. Create an app at [developer.spotify.com](https://developer.spotify.com/dashboard) and register `<your-origin>/services/spotify/callback` as the redirect URI. Not a secret — client IDs are public in OAuth. |

## Uninstalling

YourPlace uses the standard OS interface such as add/remove programs. But there is also an "Uninstall" button in [Settings > Server Info](http://localhost:42424/settings#serverInfo) that starts the same workflow

You can manually uninstall YourPlace with dedicated uninstaller scripts that YourPlace drops. Run this as an administrator to remove all YourPlace files and folders from your system.

`C:\ProgramData\YourPlace\uninstall.ps1` on Windows

`/Library/Application Support/YourPlace/uninstall.sh` on OSX
