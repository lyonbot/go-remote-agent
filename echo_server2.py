from http.server import BaseHTTPRequestHandler, HTTPServer
import hashlib

class SimpleRequestHandler(BaseHTTPRequestHandler):
    def do_POST(self):
        content_length = int(self.headers['Content-Length'])  # Get the size of data
        post_data = self.rfile.read(content_length)  # Read the POST data
        md5_checksum = hashlib.md5(post_data).hexdigest()
        
        # Respond with the size of the body
        response_message = f"the body size is {content_length}, md5_checksum = {md5_checksum}"

        self.send_response(200)
        self.send_header('Content-Type', 'text/plain')
        self.end_headers()
        self.wfile.write(response_message.encode('utf-8'))

def run(server_class=HTTPServer, handler_class=SimpleRequestHandler, port=8765):
    server_address = ('', port)
    httpd = server_class(server_address, handler_class)
    print(f'Starting http server on port {port}')
    httpd.serve_forever()

if __name__ == '__main__':
    run()
