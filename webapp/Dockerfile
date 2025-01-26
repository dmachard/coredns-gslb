FROM python:3.13-slim

RUN mkdir -p /app

COPY ./server.py /app/
COPY ./init.sh /app/
COPY ./requirements.txt /app/

RUN chmod +x /app/init.sh

WORKDIR /app
CMD ["/app/init.sh"]