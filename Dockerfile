FROM alpine:3.22

# Environment variables
ENV TZ=Etc/UTC
ENV ROTATION_TZ=Etc/UTC

# Install essential packages, common utilities, and set up timezone
RUN apk add --no-cache \
  tzdata \
  gnupg \
  wget \
  curl \
  ca-certificates \
  openssh-client \
  iptables \
  supervisor \
  dirmngr \
  nano \
  vim \
  busybox-extras \ 
  less \
  tcpdump \
  net-tools \
  lsof \
  libaio \   
  unzip \
  git && \
  ln -sf /usr/share/zoneinfo/${TZ} /etc/localtime && \
  echo "${TZ}" > /etc/timezone

# Install PostgreSQL client
RUN apk add --no-cache \
  postgresql16-client

# Install ClickHouse
RUN cat <<EOF > /usr/local/bin/install-clickhouse.sh
#!/bin/sh
set -eo pipefail

export VERSION=23.5.3.24

case $(uname -m) in
  x86_64) export ARCH=amd64 ;;
  aarch64) export ARCH=arm64 ;;
  *) echo "Unknown architecture $(uname -m)"; exit 1
esac

for PKG in clickhouse-common-static clickhouse-client; do
  curl -fO "https://packages.clickhouse.com/tgz/stable/\$PKG-\$VERSION-\${ARCH}.tgz" || curl -fO "https://packages.clickhouse.com/tgz/stable/\$PKG-\$VERSION.tgz"
done

tar -xzvf "clickhouse-common-static-\$VERSION-\${ARCH}.tgz" || tar -xzvf "clickhouse-common-static-\$VERSION.tgz"
clickhouse-common-static-\$VERSION/install/doinst.sh

tar -xzvf "clickhouse-client-\$VERSION-\${ARCH}.tgz" || tar -xzvf "clickhouse-client-\$VERSION.tgz"
clickhouse-client-\$VERSION/install/doinst.sh
EOF

RUN chmod +x /usr/local/bin/install-clickhouse.sh && /usr/local/bin/install-clickhouse.sh && rm -f /usr/local/bin/install-clickhouse.sh

# Create a non-root user and group
# Alpine uses -S for system users/groups.
RUN addgroup -S trcligroup && \
  adduser -S -G trcligroup trcliuser

# Copy the Go binary 
COPY trcli /usr/local/bin/trcli

# Set executable permission
RUN chmod +x /usr/local/bin/trcli

# Switch to the non-root user 
USER trcliuser

# Switch back to root before ENTRYPOINT
USER root

# Entrypoint 
ENTRYPOINT ["/usr/local/bin/trcli"]