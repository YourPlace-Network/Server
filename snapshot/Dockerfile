FROM ubuntu:24.04 AS builder

RUN apt-get update && apt-get install -y \
    git wget ca-certificates make \
    && rm -rf /var/lib/apt/lists/*

RUN wget https://go.dev/dl/go1.25.1.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go1.25.1.linux-amd64.tar.gz && \
    rm go1.25.1.linux-amd64.tar.gz

ENV PATH="/usr/local/go/bin:${PATH}"

WORKDIR /build

RUN git clone https://github.com/YourPlace-Network/Server.git . && \
    git fetch --tags && \
    LATEST_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "main") && \
    git checkout ${LATEST_TAG}

RUN make snapshot_build

FROM ubuntu:24.04

RUN apt-get update && apt-get install -y \
    ca-certificates tzdata unattended-upgrades \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /build/target/YourPlaceSnapshot .

ENV BASE_RPC_URL="" \
    BASE_RPC_THROTTLE="5" \
    S3_ENDPOINT="" \
    S3_BUCKET_NAME="" \
    S3_ACCESS_KEY="" \
    S3_SECRET_KEY=""

VOLUME ["/root/YourPlaceSnapshot"]

CMD ["./YourPlaceSnapshot"]