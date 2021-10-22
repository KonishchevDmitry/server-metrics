FROM golang AS build
WORKDIR /go/src/app

COPY . .
ENV CGO_ENABLED=0 GOCACHE=/cache/go GOMODCACHE=/cache/mod
RUN --mount=type=cache,id=go,target=/cache \
    go test -mod=readonly -v ./...
RUN --mount=type=cache,id=go,target=/cache \
    go install -mod=readonly ./cmd/server-metrics

FROM scratch
COPY --from=build /go/bin/server-metrics /server-metrics
EXPOSE 9101
ENTRYPOINT ["/server-metrics"]