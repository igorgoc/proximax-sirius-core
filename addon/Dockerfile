ARG BUILD_FROM=ghcr.io/home-assistant/aarch64-base-debian:bookworm
FROM ${BUILD_FROM}

ENV LANG=C.UTF-8

# Install runtime dependencies for Sirius Catapult engine
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    tar \
    zstd \
    jq \
    libatomic1 \
    libssl3 \
    libsnappy1v5 \
    libgflags2.2 \
    procps \
    && rm -rf /var/lib/apt/lists/*

# Stage configuration templates and assets
WORKDIR /etc/sirius
COPY chainconfig /etc/sirius/chainconfig

# Copy entrypoint script
COPY run.sh /run.sh
RUN chmod a+x /run.sh

# Expose P2P network port
EXPOSE 7900

ENTRYPOINT ["/run.sh"]
