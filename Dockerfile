# syntax=docker/dockerfile:1.7
#
# Optional container build for kubectl-srepulse. Most users will
# install via Krew or `go install`; this image exists for CI runners
# and folks who'd rather call the CLI from a container in a Job.
#
# Multi-arch: use scripts/docker-multiarch.sh in srepulse/website as
# the reference shape. Build stages pin to $BUILDPLATFORM so the
# Go compiler runs on host arch; the final stage cross-builds.

ARG GO_VERSION=1.23-alpine
ARG ALPINE_VERSION=3.20

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS build
WORKDIR /src

ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown
ARG TARGETOS
ARG TARGETARCH

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath \
      -ldflags "-s -w -X main.version=$VERSION -X main.commit=$COMMIT -X main.date=$DATE" \
      -o /out/kubectl-srepulse \
      ./cmd/kubectl-srepulse

FROM alpine:${ALPINE_VERSION}
LABEL org.opencontainers.image.title="kubectl-srepulse"
LABEL org.opencontainers.image.description="Terminal client for srepulse — autonomous Kubernetes SRE."
LABEL org.opencontainers.image.source="https://github.com/srepulse/cli"
LABEL org.opencontainers.image.licenses="Apache-2.0"

RUN apk add --no-cache ca-certificates && adduser -D -u 1000 srepulse
USER srepulse

COPY --from=build /out/kubectl-srepulse /usr/local/bin/kubectl-srepulse

ENTRYPOINT ["/usr/local/bin/kubectl-srepulse"]
CMD ["--help"]
