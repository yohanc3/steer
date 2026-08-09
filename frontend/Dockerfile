FROM node:22-alpine AS build

WORKDIR /app

COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

COPY frontend/ ./

ARG VITE_TELLER_APPLICATION_ID
ARG VITE_TELLER_ENVIRONMENT

ENV VITE_TELLER_APPLICATION_ID=$VITE_TELLER_APPLICATION_ID
ENV VITE_TELLER_ENVIRONMENT=$VITE_TELLER_ENVIRONMENT
ENV VITE_SERVER_BASE_URL=""

RUN npm run build

FROM nginxinc/nginx-unprivileged:1.27-alpine

COPY frontend/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html

EXPOSE 8080
