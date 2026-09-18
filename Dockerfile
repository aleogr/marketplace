# The image carries the statically linked binary and nothing else: no shell, no
# package manager, and a non-root user, so a vulnerability scan has almost no
# surface to report and an intrusion has almost no tools to use.

FROM golang:1.26.6 AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG VERSION=unknown
RUN CGO_ENABLED=0 go build \
      -trimpath \
      -ldflags "-s -w -X github.com/aleogr/marketplace/internal/platform/version.version=${VERSION}" \
      -o /out/marketplace ./cmd/marketplace

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/marketplace /marketplace
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/marketplace"]
