FROM ubuntu:24.04 AS builder

RUN apt-get update && apt-get install -y \
    wget ca-certificates make gcc libpcap-dev \
    && rm -rf /var/lib/apt/lists/*

RUN wget https://go.dev/dl/go1.25.1.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go1.25.1.linux-amd64.tar.gz && \
    rm go1.25.1.linux-amd64.tar.gz

ENV PATH="/usr/local/go/bin:${PATH}"

WORKDIR /build

COPY . .

RUN mkdir -p target/ && \
    GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o target/YourPlaceGateway main.go

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

COPY --from=builder /build/target/YourPlaceGateway .

VOLUME ["/root/YourPlace"]

EXPOSE 22 443

ENTRYPOINT ["/entrypoint.sh"]
CMD ["/usr/bin/supervisord", "-c", "/etc/supervisor/conf.d/supervisord.conf"]
