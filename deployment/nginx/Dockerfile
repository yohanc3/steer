FROM nginx:1.27-alpine

RUN apk add --no-cache certbot

COPY deployment/nginx/reload-on-renewal.sh /usr/local/bin/reload-on-renewal

RUN chmod 755 /usr/local/bin/reload-on-renewal

ENTRYPOINT ["/usr/local/bin/reload-on-renewal"]
