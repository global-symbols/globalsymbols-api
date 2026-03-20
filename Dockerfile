FROM golang:1.25-alpine AS build

WORKDIR /app

# Preload module definitions
COPY go.mod ./
RUN go mod download

# Copy the full source tree
COPY . .

# Ensure all imported modules are fetched and go.sum is populated
RUN go get ./...

# Build static binary for Linux
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /go-api ./cmd/api

FROM alpine:3.19

WORKDIR /app

COPY --from=build /go-api /app/go-api

EXPOSE 8080

ENV PORT=8080

CMD ["/app/go-api"]

