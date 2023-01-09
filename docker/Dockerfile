FROM golang:1.19.4-bullseye AS Builder
ENV GOPROXY=https://goproxy.cn
ENV CGO_ENABLED=0
WORKDIR /app
COPY . .
RUN go mod download -x
RUN GOOS=linux GOARCH=amd64 go build -ldflags "-w -s"  -o webhook
FROM debian:stretch
LABEL maintainer "soulteary <soulteary@gmail.com>"
COPY --from=builder /app/webhook /bin/
CMD webhook