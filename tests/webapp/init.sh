#!/bin/sh

# docker-compose depends_on is still too fast sometimes
# adding a sleep 15s to make sure gen_certs is complete
sleep 15s

APP_NAME=${APP_NAME:-WebApp} # WebApplication1[0-2]
cn_san=$(echo $APP_NAME | tr '[:upper:]' '[:lower:]')
APP_IP=$(hostname -I)

if [ ! -f "/certs/$cn_san/cert.pem" ]; then
  mkdir -p "/certs/$cn_san"
  # Create server certificate w/ extensions for validating
  # presented client identity certificate and sign w/ root CA (mTLS)
  #
  openssl req -x509 -new -newkey ED25519 -noenc -config /app/certs/rootca.cnf \
   -subj "/CN=$cn_san" -keyout "/certs/$cn_san/key.pem" \
   -out "/certs/$cn_san/cert.pem" -CA /app/certs/ca/cert.pem \
   -CAkey /app/certs/ca/key.pem -extensions server_req_exts \
   -addext "subjectAltName=DNS:$cn_san,DNS:cdns-gslb-server,DNS:localhost,IP:$APP_IP" -quiet
  cat /app/certs/ca/cert.pem >> "/certs/$cn_san/cert.pem"
fi

python /app/server.py --cafile /app/certs/ca/cert.pem --certfile "/certs/$cn_san/cert.pem" --keyfile "/certs/$cn_san/key.pem" --name "$APP_NAME"
