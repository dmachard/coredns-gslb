import http.server
import ssl
import argparse

from http.server import  BaseHTTPRequestHandler


# Parse command line arguments
parser = argparse.ArgumentParser(description="HTTPS Server")
parser.add_argument('--port', type=int, default=443, help='Port to listen on')
parser.add_argument('--certfile', type=str, required=True, help='Path to SSL certificate file')
parser.add_argument('--keyfile', type=str, required=True, help='Path to SSL key file')
parser.add_argument('--name', type=str, required=True, help='Name of the application')

args = parser.parse_args()

class SimpleHTTPRequestHandler(BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200)
        self.end_headers()
        msg = f"Welcome to {args.name}"
        self.wfile.write(msg.encode())

# Create the HTTP server
httpd = http.server.HTTPServer(('0.0.0.0', args.port), SimpleHTTPRequestHandler)

# Create an SSL context and wrap the socket
context = ssl.create_default_context(ssl.Purpose.CLIENT_AUTH)
context.load_cert_chain(certfile=args.certfile, keyfile=args.keyfile)

# Apply SSL context to the server's socket
httpd.socket = context.wrap_socket(httpd.socket, server_side=True)

# Start the server
httpd.serve_forever()
