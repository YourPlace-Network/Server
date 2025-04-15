# YourPlace
## Distributed Social Media
#### ⚠️ Code In Development - Expect Small Bugs ⚠️
YourPlace is a distributed social media platform that is designed to give complete sovereignty back to users. You own your content creation, publishing, consumption and follower relationships.

The core of YourPlace is a network of self-hosted servers called "places." You can keep your place on your laptop, a cloud server, a 3rd party service, or wherever you can run a PC.

Places are owned and managed by a blockchain wallet address. They serve as a hub into your social life. Servers are where you host your profile, read other peoples posts, and share your content. YourPlace servers act on your behalf to manage your entire social media life.


### Running YourPlace

Download the YourPlace binary for your OS and run it.

Please see our [Terms of Service](https://github.com/YourPlace-Network/Server/blob/main/TOS.md) and [Privacy Policy](https://github.com/YourPlace-Network/Server/blob/main/PRIVACY.md) for more information on your responsibilities as a user and how we handle your data.

### [⬇️ DOWNLOAD NOW ⬇️](https://YourPlace.network/download)

Upon first run, YourPlace will open up a local setup page which will guide you through the rest of the installation.

Desktop shortcuts will be created after install, or you can visit the [main local interface](http://localhost:42424/) running on your PC.

### Building

YourPlace server builds on Windows x64, OSX Apple Silicon, and coming soon: Linux x64

Install [Make](https://www.gnu.org/software/make/), [Go](https://go.dev) and [Node.js](https://nodejs.org) on your system. Then run the command

`make clean install build`

To run a build with debugging enabled, run the command

`make clean install dbg_build dbg_run`

Output YourPlace binary will be in the `target/` directory

See the [Makefile](https://github.com/YourPlace-Network/Server/blob/main/Makefile) for more build targets
