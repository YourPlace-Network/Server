# YourPlace Gateway

## Build

### Desktop Binary (with UI)
```bash
make clean install build
```

### Gateway Binary (headless, no GTK)
```bash
make gateway_build
```

Gateway binary is:
- Fully static (CGO disabled)
- No desktop dependencies (systray, GTK)
- Optimized for headless servers

## Run Locally

### Desktop Mode
```bash
./target/YourPlace-<version>
```

### Gateway Mode
```bash
./target/YourPlace-<version> -g -d -du
```

Flags:
- `-g` - Enable gateway mode
- `-d` - Debug logging
- `-du` - Disable UI (don't auto-open browser)
- `-di` - Disable blockchain indexer

## Gateway Features

Gateway mode enables:
- TLS server on port 443 (requires `cert.pem` and `cert.key` in data directory)
- Cloudflare proxy header trust (`router.TrustedPlatform = gin.PlatformCloudflare`)
- Headless operation (no systray)
- Restricted endpoints (see middleware/gateway.go:10)

Gateway mode disables:
- File uploads (POST /files)
- Settings changes (POST /settings)
- Setup operations (GET/POST /setup)
- Notifications (POST /notifications)
- Desktop UI features

## Docker Deployment

See `gateway/README.md` for Docker and AWS deployment instructions.

## Use Cases

**Gateway Mode:**
- Public YourPlace instance
- Cloud deployment (AWS, DigitalOcean, etc.)
- Reverse proxy behind Cloudflare
- Headless servers (no display)

**Desktop Mode:**
- Local development
- Personal YourPlace server
- Desktop applications with system tray