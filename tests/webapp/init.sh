#!/bin/sh

# ensure the root ca cert exists before continuing
while [ $(find /app/certs/ca/ -type f -name "*.pem" | wc -l) -lt 2 ]
do
  sleep 2s
done

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
