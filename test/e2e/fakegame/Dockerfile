# Fake game server for the e2e backup test. Build from the repository root:
#   docker build -f test/e2e/fakegame/Dockerfile .
# Base images match the operator's Dockerfile, so Renovate updates both in one PR.
FROM --platform=$BUILDPLATFORM golang:1.27@sha256:f20d83a83e18dbd2a4714b8ffd4b30b287b6243962dd75608e86abcc63ba4efc AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
COPY go.mod go.sum ./
COPY test/e2e/fakegame/ test/e2e/fakegame/
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -o fakegame ./test/e2e/fakegame

FROM gcr.io/distroless/static:nonroot@sha256:18f4cc26c9ffa7f20a359298be7deadac5c607c8e03a02b8119f1a5fc2f82c99
COPY --from=builder /workspace/fakegame /fakegame
USER 65532:65532

ENTRYPOINT ["/fakegame"]
