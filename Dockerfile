# syntax = docker/dockerfile:experimental

FROM golang AS build
WORKDIR /go/src/app

COPY . .
RUN --mount=type=cache,id=go,target=/cache \
    CGO_ENABLED=0 GOCACHE=/cache/go GOMODCACHE=/cache/mod \
    go install -mod=readonly ./cmd/server-metrics

FROM scratch
COPY --from=build /go/bin/server-metrics /server-metrics
USER 1:1
ENTRYPOINT ["/server-metrics"]