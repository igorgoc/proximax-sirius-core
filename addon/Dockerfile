ARG BUILD_FROM=ubuntu:24.04
FROM ${BUILD_FROM}

ENV LANG=C.UTF-8

# Install runtime dependencies for Sirius Catapult engine (Ubuntu 24.04 Noble with GLIBC 2.38+ & GCC 13+)
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    tar \
    zstd \
    jq \
    libatomic1 \
    libssl3 \
    libsnappy1v5 \
    procps \
    && apt-get install -y --no-install-recommends software-properties-common \
    && add-apt-repository -y universe 2>/dev/null || true \
    && apt-get update && apt-get install -y --no-install-recommends libgflags2.2 \
    && rm -rf /var/lib/apt/lists/*

# Stage configuration templates and assets
WORKDIR /etc/sirius
COPY chainconfig /etc/sirius/chainconfig

# Copy entrypoint script
COPY run.sh /run.sh
RUN chmod a+x /run.sh

# Expose P2P network ports
EXPOSE 7900 7901 7902 7903 7904

ENTRYPOINT ["/run.sh"]
