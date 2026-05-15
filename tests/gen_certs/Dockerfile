FROM python:3.13-slim

RUN mkdir -p /app/gen_certs

COPY ./rootca.cnf /app/gen_certs/
COPY ./gencerts.sh /app/
RUN chmod +x /app/gencerts.sh

WORKDIR /app
CMD ["/app/gencerts.sh"]