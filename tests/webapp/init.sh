#!/bin/sh

if [ ! -f /app/certs/cert.pem ]; then
  mkdir -p /app/certs
  openssl req -x509 -newkey rsa:4096 -keyout /app/certs/key.pem -out /app/certs/cert.pem -days 365 -nodes -subj '/CN=localhost'
fi

APP_NAME=${APP_NAME:-WebApp}

exec python /app/server.py --certfile /app/certs/cert.pem --keyfile /app/certs/key.pem --name "$APP_NAME"
