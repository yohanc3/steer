FROM python:3.13-alpine

WORKDIR /app

COPY deployment/predeployment/predeployment /app/predeployment

ENTRYPOINT ["python", "-m", "predeployment"]
