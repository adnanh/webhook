FROM golang:latest

MAINTAINER Dean dean@airdb.com
# github webhook:  github.com/adnanh/webhook

ENV PATH="$PATH:/go/bin"
RUN go get github.com/adnanh/webhook

RUN cd /go/src/github.com/adnanh/webhook

ENTRYPOINT webhook  -verbose -hooks ./hooks.json.example

EXPOSE 9000
