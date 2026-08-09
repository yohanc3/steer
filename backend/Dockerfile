FROM golang:1.25.5-alpine AS build

WORKDIR /src

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./
RUN CGO_ENABLED=0 go build -o /out/steer .

FROM alpine:3.21

RUN apk add --no-cache ca-certificates \
    && adduser -D -H -u 10001 steer

COPY --from=build /out/steer /usr/local/bin/steer

USER steer

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/steer"]
