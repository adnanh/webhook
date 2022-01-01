FROM golang:1.17.5-stretch AS Builder
WORKDIR /app
COPY . .
RUN go mod download -x
RUN GOOS=linux GOARCH=amd64 go build -o webhook

FROM debian:stretch
LABEL maintainer "soulteary <soulteary@gmail.com>"
COPY --from=builder /app/webhook /bin/
CMD webhook