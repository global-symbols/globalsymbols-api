FROM golang:1.25-alpine

RUN apk add --no-cache bash git ca-certificates curl && update-ca-certificates

WORKDIR /app

# This container is intended for interactive test runs.
# Source code will be mounted at /app by docker-compose.
CMD ["sleep", "infinity"]

