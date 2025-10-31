# IMPORTANT: Always use AWS ECR Public images to avoid DockerHub rate limits
# AWS ECR Public Gallery: https://gallery.ecr.aws/
FROM public.ecr.aws/ubuntu/ubuntu:24.04 AS downloader

RUN apt-get update && apt-get install -y \
    wget ca-certificates grep \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /build

# Copy only main.go to extract version
COPY main.go .

# Extract version from main.go and download pre-built Linux binary from DigitalOcean Spaces
RUN VERSION=$(grep -oE '^\s*version\s*=\s*"[^"]*"' main.go | grep -oE '"[^"]*"' | tr -d '"') && \
    echo "Downloading YourPlace-${VERSION}" && \
    wget https://yourplace-downloads.nyc3.digitaloceanspaces.com/linux/YourPlace-${VERSION} -O YourPlaceGateway && \
    chmod +x YourPlaceGateway

FROM public.ecr.aws/ubuntu/ubuntu:24.04

RUN apt-get update && apt-get install -y \
    ca-certificates tzdata \
    libpcap0.8 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=downloader /build/YourPlaceGateway .
COPY gateway/entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

VOLUME ["/opt/YourPlace"]

EXPOSE 443

ENTRYPOINT ["/app/entrypoint.sh"]
