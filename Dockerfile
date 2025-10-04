FROM golang AS build
WORKDIR /go/src/app

COPY . .
ENV CGO_ENABLED=0 GOCACHE=/cache/go GOMODCACHE=/cache/mod

# The tags reduce Podman bindings dependencies:
# * containers_image_docker_daemon_stub, containers_image_openpgp – https://github.com/containers/image/blob/df7e80d2d19872b61f352a8a182ec934dc0c2346/README.md?plain=1#L63
# * remote – https://github.com/containers/podman/blob/ef584d4d6d5b50d7eb48a785be6c954bb0abe5c6/pkg/bindings/README.md?plain=1#L243
ENV GOFLAGS=-tags=containers_image_docker_daemon_stub,containers_image_openpgp,remote

RUN --mount=type=cache,id=go,target=/cache \
    go test -mod=readonly -v ./...
RUN --mount=type=cache,id=go,target=/cache \
    go install -mod=readonly ./cmd/server-metrics

FROM scratch
COPY --from=build /go/bin/server-metrics /server-metrics
EXPOSE 9101
ENTRYPOINT ["/server-metrics"]