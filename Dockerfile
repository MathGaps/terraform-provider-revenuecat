# The mock RevenueCat API server.
#
# Building from the module root keeps the image using exactly the handler the
# Go tests mount in-process, so the two cannot drift apart.

FROM golang:1.26-alpine AS build
WORKDIR /src

# Copy the manifests first so dependency download is cached separately from
# source changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/mock-revenuecat ./cmd/mock-revenuecat

FROM alpine:3.20
RUN adduser -D -u 10001 mock
COPY --from=build /out/mock-revenuecat /usr/local/bin/mock-revenuecat
USER mock
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/mock-revenuecat"]
CMD ["-addr", ":8080"]
