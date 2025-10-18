FROM ubuntu:24.04 AS downloader

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

FROM ubuntu:24.04

RUN apt-get update && apt-get install -y \
    ca-certificates tzdata unattended-upgrades openssh-server supervisor \
    && rm -rf /var/lib/apt/lists/*

# Configure SSH for root login with key authentication only
RUN mkdir /var/run/sshd && \
    sed -i 's/#PermitRootLogin prohibit-password/PermitRootLogin prohibit-password/' /etc/ssh/sshd_config && \
    sed -i 's/#PasswordAuthentication yes/PasswordAuthentication no/' /etc/ssh/sshd_config && \
    sed -i 's/#PubkeyAuthentication yes/PubkeyAuthentication yes/' /etc/ssh/sshd_config && \
    mkdir -p /root/.ssh && \
    chmod 700 /root/.ssh && \
    touch /root/.ssh/authorized_keys && \
    chmod 600 /root/.ssh/authorized_keys

# Configure supervisor to run both SSH and the gateway
RUN mkdir -p /var/log/supervisor
COPY gateway/supervisord.conf /etc/supervisor/conf.d/supervisord.conf

# Create entrypoint script to setup SSH key from environment variable
COPY gateway/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

WORKDIR /app

COPY --from=downloader /build/YourPlaceGateway .

VOLUME ["/root/YourPlace"]

EXPOSE 22 443

ENTRYPOINT ["/entrypoint.sh"]
CMD ["/usr/bin/supervisord", "-c", "/etc/supervisor/conf.d/supervisord.conf"]
