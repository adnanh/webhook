FROM golang:1.15 AS builder
WORKDIR /go/src/github.com/adnanh/webhook/
COPY . ./
RUN make CGO_ENABLED=0 LDFLAGS="-w -s" build

FROM scratch
COPY --from=builder /go/src/github.com/adnanh/webhook/webhook /bin/
ENTRYPOINT ["/bin/webhook"]
