# YourPlace Gateway

## Running your own gateway

The YourPlace binary has a mode where it allows gateway functionality, in addition to being a YourPlace server.
Command: ``YourPlace.exe -g`` When run in this mode, new gateway pages and functionality are exposed in the local web UI.

### Pros

* Fully distributed, self-hosted access to browse YourPlace profiles and posts
* Low-powered content searching: can run on a raspberryPi

### Cons

* Limited text search: Algorand indexers only support searching by prefix, which prevents hashtags and rich-text searching of posts and profiles
* No content feeds: Recommended content feeds and block lists may not be available in a self-hosted gateway

### Gateway Mode Changes

* Sets `router.TrustedPlatform = gin.PlatformCloudflare` so that the router knows to trust Cloudflare's headers
* * Assumes it'll be running locally or behind Cloudflare)
* Enables TLS server on port *:443 which expects a Cloudflare origin key `<data_directory>\cert.key` and PEM certificate `<data_directory>\cert.pem`
* Hides settings link in frontend menu
* Sets environment variable `YourPlaceGateway=true`
* Disable file, banner and avatar uploads (gateway.go middleware)