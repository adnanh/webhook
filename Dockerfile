FROM golang:1.5.1-onbuild

MAINTAINER Zack YL Shih <zackyl.shih@moxa.com>

EXPOSE 9000

ENTRYPOINT ["/usr/local/go/bin/go", "run", "webhook.go"]