# Generic multi-stage build for every Unital Go service.
# Build arg CMD selects the package path, e.g. `gateway` or
# `services/identity/cmd/server`.
FROM golang:1.26-alpine AS build
ARG CMD
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download || true
COPY . .
RUN CGO_ENABLED=0 go build -o /app "./${CMD}"

FROM alpine:3.20
RUN adduser -D -u 10001 app
USER app
COPY --from=build /app /app
ENTRYPOINT ["/app"]
