FROM golang:1.19.4-bullseye AS Builder
WORKDIR /app
COPY . .
RUN go mod download -x
RUN GOOS=linux GOARCH=amd64 go build -o webhook

FROM debian:stretch
LABEL maintainer "soulteary <soulteary@gmail.com>"
COPY --from=builder /app/webhook /bin/
CMD webhook