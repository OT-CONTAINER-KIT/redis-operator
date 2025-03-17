# Build the manager or agent binary
FROM golang:1.23-alpine AS builder
ARG BUILDOS
ARG BUILDPLATFORM
ARG BUILDARCH
ARG BUILDVARIANT
ARG TARGETPLATFORM
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

# Define build target - can be "manager" or "agent"
ARG BUILD_TARGET=manager

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY cmd/${BUILD_TARGET}/main.go main.go
COPY api/ api/
COPY pkg/ pkg/
COPY mocks/ mocks/

# Build
ARG LDFLAGS="-s -w"
ENV GOOS=$TARGETOS
ENV GOARCH=$TARGETARCH
ENV CGO_ENABLED=0

RUN GO111MODULE=on go build -ldflags "${LDFLAGS}" -a -o ${BUILD_TARGET} main.go

# Use distroless as minimal base image to package the binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
LABEL maintainer="The Opstree Opensource <opensource@opstree.com>"
WORKDIR /

ARG BUILD_TARGET=manager
COPY --from=builder /workspace/${BUILD_TARGET} /${BUILD_TARGET}
USER 65532:65532

ENTRYPOINT ["/${BUILD_TARGET}"] 