# Build/test toolchain for the Homebox CI pipeline.
#
# The production image (../Dockerfile) is multi-stage and self-sufficient, so
# this image is only used by the Test stages. It exists because Jenkins runs
# inside a container and talks to the host Docker daemon: a bind mount of the
# workspace would reference a path that does not exist on the host. Letting the
# docker-workflow plugin run the steps in this image sidesteps that entirely.
FROM golang:1.23-alpine

# build-base: the repo's tests use mattn/go-sqlite3, which needs cgo and a C
# compiler. The production binary is built CGO-free and does not.
RUN apk add --no-cache \
      build-base \
      git \
      curl \
      bash \
      nodejs \
      npm

# Pinned: the lockfile is v9 and pnpm 10 refuses to run the build scripts this
# project needs.
RUN npm install -g pnpm@9

# Fail the image build rather than a pipeline run if a tool is missing.
RUN go version && node --version && npm --version && pnpm --version && gcc --version | head -1

# Jenkins starts the container with -u 1000:1000, and that uid has no $HOME in
# this image. Go, npm and pnpm all want to write caches and would crash. Point
# HOME and every cache at world-writable /tmp.
ENV HOME=/tmp \
    NPM_CONFIG_CACHE=/tmp/.npm \
    PNPM_HOME=/tmp/.pnpm \
    GOCACHE=/tmp/.cache/go-build \
    GOMODCACHE=/tmp/.cache/go-mod \
    GOFLAGS=-buildvcs=false \
    CI=true
RUN mkdir -p /tmp/.npm /tmp/.pnpm /tmp/.cache/go-build /tmp/.cache/go-mod \
 && chmod -R 1777 /tmp

WORKDIR /workspace
