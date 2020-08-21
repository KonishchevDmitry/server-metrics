FROM golang AS build
WORKDIR /go/src/app

COPY go.mod go.sum ./
RUN go mod download -x

COPY . .
RUN CGO_ENABLED=0 go install -mod=readonly ./cmd/server-metrics

FROM scratch
COPY --from=build /go/bin/server-metrics /server-metrics
USER 1:1
ENTRYPOINT ["/server-metrics"]