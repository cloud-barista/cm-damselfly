##############################################################
## Stage 1 - Go Build
##############################################################

FROM golang:1.23.0-bookworm AS builder

ENV GO111MODULE=on

# Set the Current Working Directory inside the container
WORKDIR /go/src/github.com/cloud-barista/cm-damselfly

# Copy dependency files to the container
COPY go.mod go.sum go.work go.work.sum LICENSE ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

# Copy some necessary files to the container
COPY api ./api
COPY cmd/cm-damselfly ./cmd/cm-damselfly
# COPY conf ./conf
COPY pkg ./pkg

# Build the Go app
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    cd cmd/cm-damselfly && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags '-s -w' -tags cm-damselfly -v -o cm-damselfly main.go

#############################################################
## Stage 2 - Application Setup
##############################################################

FROM ubuntu:22.04 AS prod

RUN rm /bin/sh && ln -s /bin/bash /bin/sh

# Set the Current Working Directory inside the container
WORKDIR /app

# Installing necessary packages and cleaning up
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

## Copy the Pre-built binary and necessary files from the previous stage
COPY --from=builder /go/src/github.com/cloud-barista/cm-damselfly/api/ /app/api/
COPY --from=builder /go/src/github.com/cloud-barista/cm-damselfly/cmd/cm-damselfly/cm-damselfly /app/
# COPY --from=builder /go/src/github.com/cloud-barista/cm-damselfly/conf/ /app/conf/

## To get the version of go modules
COPY --from=builder /go/src/github.com/cloud-barista/cm-damselfly/go.mod /app/

## To prevent the Golang Error: 'missing Location in call to Time.In'
# Copy timezone from Golang base image
COPY --from=builder /usr/local/go/lib/time/zoneinfo.zip /
# Specify timezone info location for Go application
ENV ZONEINFO=/zoneinfo.zip

## Set environment variables 
# Set system endpoints
ENV DAMSELFLY_ROOT=/app 

## Set SELF_ENDPOINT, to access Swagger API dashboard outside (Ex: export SELF_ENDPOINT=x.x.x.x:8088)
ENV DAMSELFLY_SELF_ENDPOINT=localhost:8088

## Set API access config
# API_ALLOW_ORIGINS (ex: https://cloud-barista.org,xxx.xxx.xxx.xxx or * for all)
# Set ENABLE_AUTH=true currently for basic auth for all routes (i.e., url or path)
ENV DAMSELFLY_API_ALLOW_ORIGINS=* \
    DAMSELFLY_API_AUTH_ENABLED=true \
    DAMSELFLY_API_USERNAME=default \
    DAMSELFLY_API_PASSWORD=default

## Set internal DB config (lkvstore: local key-value store, default file path: ./db/damselfly.db)
ENV DAMSELFLY_LKVSTORE_PATH=/app/db/damselfly.db

## Logger configuration
# Set log file path (default logfile path: ./damselfly.log) 
# Set log level, such as trace, debug info, warn, error, fatal, and panic
ENV DAMSELFLY_LOGFILE_PATH=/app/log/damselfly.log \
    DAMSELFLY_LOGFILE_MAXSIZE=1000 \
    DAMSELFLY_LOGFILE_MAXBACKUPS=3 \
    DAMSELFLY_LOGFILE_MAXAGE=30 \
    DAMSELFLY_LOGFILE_COMPRESS=false \
    DAMSELFLY_LOGLEVEL=info \
    DAMSELFLY_LOGWRITER=both

# Set execution environment, such as development or production
ENV DAMSELFLY_NODE_ENV=production

## Set period for auto control goroutine invocation
ENV DAMSELFLY_AUTOCONTROL_DURATION_MS=10000

ENTRYPOINT [ "/app/cm-damselfly" ]

EXPOSE 8088
