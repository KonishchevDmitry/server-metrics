FROM golangci/golangci-lint:v1.30.0
WORKDIR /go/src/app
COPY . .
RUN golangci-lint run --color always

FROM golang AS build
WORKDIR /go/src/app
COPY . .
RUN go install ./cmd/server-metrics

FROM scratch
COPY --from=build /go/bin/server-metrics /server-metrics
USER 1:1
ENTRYPOINT ["/server-metrics"]