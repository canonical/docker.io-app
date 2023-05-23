# syntax=docker/dockerfile:1

ARG BASE_VARIANT=alpine
ARG GO_VERSION=1.19.8
ARG ALPINE_VERSION=3.16
ARG XX_VERSION=1.1.0

FROM --platform=$BUILDPLATFORM tonistiigi/xx:${XX_VERSION} AS xx

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine${ALPINE_VERSION} AS build-base-alpine
COPY --from=xx / /
RUN apk add --no-cache clang lld llvm file git
WORKDIR /go/src/github.com/docker/cli

FROM build-base-alpine AS build-alpine
ARG TARGETPLATFORM
# gcc is installed for libgcc only
RUN xx-apk add --no-cache musl-dev gcc

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-buster AS build-base-buster
COPY --from=xx / /
RUN apt-get update && apt-get install --no-install-recommends -y clang lld file
WORKDIR /go/src/github.com/docker/cli

FROM build-base-buster AS build-buster
ARG TARGETPLATFORM
RUN xx-apt install --no-install-recommends -y libc6-dev libgcc-8-dev

FROM build-${BASE_VARIANT} AS build
# GO_LINKMODE defines if static or dynamic binary should be produced
ARG GO_LINKMODE=static
# GO_BUILDTAGS defines additional build tags
ARG GO_BUILDTAGS
# GO_STRIP strips debugging symbols if set
ARG GO_STRIP
# CGO_ENABLED manually sets if cgo is used
ARG CGO_ENABLED
# VERSION sets the version for the produced binary
ARG VERSION
RUN --mount=ro --mount=type=cache,target=/root/.cache \
    --mount=from=dockercore/golang-cross:xx-sdk-extras,target=/xx-sdk,src=/xx-sdk \
    --mount=type=tmpfs,target=cli/winresources \
    xx-go --wrap && \
    # export GOCACHE=$(go env GOCACHE)/$(xx-info)$([ -f /etc/alpine-release ] && echo "alpine") && \
    TARGET=/out ./scripts/build/binary && \
    xx-verify $([ "$GO_LINKMODE" = "static" ] && echo "--static") /out/docker

FROM build-base-${BASE_VARIANT} AS dev
COPY . .

FROM scratch AS binary
COPY --from=build /out .
