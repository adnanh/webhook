FROM        alpine
MAINTAINER  Adnan Hajdarevic <adnanh@gmail.com>

ENV         GOPATH /go
ENV         SRCPATH ${GOPATH}/src/github.com/adnanh/webhook
COPY        . ${SRCPATH}
RUN         apk add --update -t build-deps go git libc-dev gcc libgcc && \
            cd ${SRCPATH} && go get -d && go build -o /usr/local/bin/webhook && \
            apk del --purge build-deps && \
            rm -rf /var/cache/apk/* && \
            rm -rf ${GOPATH}

EXPOSE      9000
ENTRYPOINT  ["/usr/local/bin/webhook"]
