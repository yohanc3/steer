FROM alpine:3.21

RUN apk add --no-cache sqlite

COPY deployment/backup/backup.sh /usr/local/bin/backup
COPY deployment/backup/crontab /etc/crontabs/root

RUN chmod 755 /usr/local/bin/backup

CMD ["crond", "-f", "-l", "2", "-L", "/dev/stdout"]
