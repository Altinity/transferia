FROM quay.io/debezium/postgres:18

USER root

RUN apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
        postgresql-18-wal2json \
    && rm -rf /var/lib/apt/lists/*

USER postgres
