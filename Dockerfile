# Build the manager or agent binary
FROM golang:1.23-alpine3.21 AS builder
ARG BUILDOS
ARG BUILDPLATFORM
ARG BUILDARCH
ARG BUILDVARIANT
ARG TARGETPLATFORM
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY cmd/ cmd/
COPY api/ api/
COPY pkg/ pkg/
COPY mocks/ mocks/

# Build
ARG LDFLAGS="-s -w"
ENV GOOS=$TARGETOS
ENV GOARCH=$TARGETARCH
ENV CGO_ENABLED=0

# Build the unified binary
RUN GO111MODULE=on go build -ldflags "${LDFLAGS}" -a -o operator cmd/manager/main.go

# Use distroless as minimal base image to package the binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
LABEL maintainer="The Opstree Opensource <opensource@opstree.com>"
WORKDIR /

COPY --from=builder /workspace/operator /operator
USER 65532:65532

ENTRYPOINT ["/operator", "manager"] 