<div align = center>

<img src="https://raw.githubusercontent.com/YourPlace-Network/Server/refs/heads/main/src/www/image/yourplace-banner-title.svg" width="750" height="300" alt="banner">

<br><br>

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
[![Install Button]][Install]
[![Whitepaper Button]][Whitepaper]
[![FAQ Button]][FAQ]

---

#### ⚠️ Code In Development - Expect Small Bugs ⚠️

<br>

</div>

[Home]: https://yourplace.network
[Install]: https://yourplace.network/download
[Whitepaper]: https://whitepaper.yourplace.network
[FAQ]: https://yourplace.network/faq
[Home Button]: https://img.shields.io/badge/Home-4A90E2?style=for-the-badge&logoColor=white
[Install Button]: https://img.shields.io/badge/Install-5CB85C?style=for-the-badge&logoColor=white
[Whitepaper Button]: https://img.shields.io/badge/Whitepaper-9B59B6?style=for-the-badge&logoColor=white
[FAQ Button]: https://img.shields.io/badge/FAQ-F0AD4E?style=for-the-badge&logoColor=white

The core of YourPlace is a network of self-hosted servers called "places." You can keep your place on your laptop, a cloud server, a 3rd party service, or wherever you can run a PC.

Places are owned and managed by a blockchain wallet address. They serve as a hub into your social life. Servers are where you host your profile, read other peoples posts, and share your content. YourPlace servers act on your behalf to manage your entire social media life.


## Running YourPlace

Download the YourPlace binary for your OS and run it.

Please see our [Terms of Service](https://github.com/YourPlace-Network/Server/blob/main/TOS.md) and [Privacy Policy](https://github.com/YourPlace-Network/Server/blob/main/PRIVACY.md) for more information on your responsibilities as a user and how we handle your data.

Upon first run, YourPlace will open up a local setup page which will guide you through the rest of the installation.

Desktop shortcuts will be created after install, or you can visit the [main local interface](http://localhost:42424/) running on your PC.

## Building

YourPlace server builds on Windows x64, OSX Apple Silicon, and coming soon: Linux x64

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

## Uninstalling

YourPlace uses the standard OS interface such as add/remove programs. But there is also an "Uninstall" button in [Settings > Server Info](http://localhost:42424/settings#serverInfo) that starts the same workflow

You can manually uninstall YourPlace with dedicated uninstaller scripts that YourPlace drops. Run this as an administrator to remove all YourPlace files and folders from your system.

`C:\ProgramData\YourPlace\uninstall.ps1` on Windows

`/Library/Application Support/YourPlace/uninstall.sh` on OSX
