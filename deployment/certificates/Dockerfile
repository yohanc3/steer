FROM alpine:3.21

RUN apk add --no-cache openssl

COPY deployment/certificates/prepare.sh /usr/local/bin/prepare-certificate

RUN chmod 755 /usr/local/bin/prepare-certificate

ENTRYPOINT ["/usr/local/bin/prepare-certificate"]
