import os
import sys
import threading
import http.server
import ssl
import argparse
from concurrent import futures
import json

# gRPC imports
try:
    import grpc
    from grpc_health.v1 import health, health_pb2_grpc
    GRPC_AVAILABLE = True
except ImportError:
    GRPC_AVAILABLE = False

from http.server import BaseHTTPRequestHandler

def run_https_server(port, name, certfile, keyfile):
    class SimpleHTTPRequestHandler(BaseHTTPRequestHandler):
        def do_GET(self):
            if self.path == "/api/health":
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.end_headers()
                resp = {"status": "green", "number_of_nodes": 3}
                self.wfile.write(json.dumps(resp).encode())
            else:
                self.send_response(200)
                self.end_headers()
                msg = f"Welcome to {name}"
                self.wfile.write(msg.encode())

    httpd = http.server.HTTPServer(('0.0.0.0', port), SimpleHTTPRequestHandler)
    context = ssl.create_default_context(ssl.Purpose.CLIENT_AUTH)
    context.load_cert_chain(certfile=certfile, keyfile=keyfile)
    httpd.socket = context.wrap_socket(httpd.socket, server_side=True)
    httpd.serve_forever()

def run_grpc_health_server(port):
    import grpc
    from grpc_health.v1 import health, health_pb2_grpc, health_pb2
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=2))
    health_servicer = health.HealthServicer()
    health_pb2_grpc.add_HealthServicer_to_server(health_servicer, server)
    health_servicer.set('', health_pb2.HealthCheckResponse.SERVING)
    server.add_insecure_port(f'[::]:{port}')
    server.start()
    server.wait_for_termination()

def main():
    parser = argparse.ArgumentParser(description="HTTPS and optional gRPC Health Server")
    parser.add_argument('--port', type=int, default=443, help='Port to listen on (HTTPS)')
    parser.add_argument('--certfile', type=str, required=True, help='Path to SSL certificate file')
    parser.add_argument('--keyfile', type=str, required=True, help='Path to SSL key file')
    parser.add_argument('--name', type=str, required=True, help='Name of the application')
    parser.add_argument('--grpc-port', type=int, default=9090, help='Port for gRPC health server')
    args = parser.parse_args()

    enable_grpc = os.environ.get('ENABLE_GRPC_HEALTH', '0') == '1'

    threads = []
    t1 = threading.Thread(target=run_https_server, args=(args.port, args.name, args.certfile, args.keyfile))
    t1.start()
    threads.append(t1)

    if enable_grpc:
        if not GRPC_AVAILABLE:
            print("gRPC health server requested but grpcio and grpcio-health-checking are not installed.", file=sys.stderr)
            sys.exit(1)
        t2 = threading.Thread(target=run_grpc_health_server, args=(args.grpc_port,))
        t2.start()
        threads.append(t2)

    for t in threads:
        t.join()

if __name__ == "__main__":
    main()
