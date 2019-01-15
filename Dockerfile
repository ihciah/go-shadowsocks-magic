FROM golang:1.11.3-alpine3.8 AS builder

RUN apk upgrade \
    && apk add git \
    && go get -ldflags '-w -s' \
        github.com/ihciah/go-shadowsocks-magic

FROM alpine:3.8

LABEL maintainer="ihciah <ihciah@gmail.com>"

RUN apk upgrade \
    && apk add bash tzdata \
    && rm -rf /var/cache/apk/*

COPY --from=builder /go/bin/go-shadowsocks-magic /usr/bin/shadowsocks

ENTRYPOINT ["shadowsocks"]
