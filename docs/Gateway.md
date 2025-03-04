# YourPlace Gateway

### Running your own gateway
The YourPlace binary has a mode where it allows gateway functionality, in addition to being a YourPlace server.
Command: ``YourPlace.exe -m gateway`` When run in this mode, new gateway pages and functionality are exposed in the local web UI.
#### Pros
* Fully distributed, self-hosted access to browse YourPlace profiles and posts
* Low-powered content searching: can run on a raspberryPi
#### Cons
* Limited text search: Algorand indexers only support searching by prefix, which prevents hashtags and rich-text searching of posts and profiles
* No content feeds: Recommended content feeds and block lists may not be available in a self-hosted gateway