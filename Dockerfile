# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.26.2
ARG DOCKER_CLI_IMAGE=docker:28-cli
ARG DISTROLESS_IMAGE=gcr.io/distroless/static-debian12:nonroot

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS build
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -buildvcs=false \
    -ldflags="-s -w -X github.com/room215/limier/cmd.version=${VERSION}" \
    -o /out/limier .

FROM ${DOCKER_CLI_IMAGE} AS docker-cli

FROM ${DISTROLESS_IMAGE} AS runtime
ARG VERSION=dev

LABEL org.opencontainers.image.title="Limier"
LABEL org.opencontainers.image.description="Fixture-based dependency behavior review tool"
LABEL org.opencontainers.image.version="${VERSION}"

ENV HOME=/tmp \
    DOCKER_CONFIG=/tmp/.docker

WORKDIR /workspace

COPY --from=build --chown=65532:65532 /out/limier /usr/local/bin/limier
COPY --from=docker-cli --chown=65532:65532 /usr/local/bin/docker /usr/local/bin/docker

USER 65532:65532

ENTRYPOINT ["/usr/local/bin/limier"]
CMD ["version"]
