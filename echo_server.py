'''
curl -X POST http://127.0.0.1:8080/api/proxy/foobar.com/ -F "agent_name=bot1" -F "target=http://127.0.0.1:8000" -F "replace_host=localhost:8000"
curl -X POST http://127.0.0.1:8080/ -H "Host: foobar.com" -d 'hello world' -v
'''

from http.server import BaseHTTPRequestHandler, HTTPServer

class SimpleHTTPRequestHandler(BaseHTTPRequestHandler):
    
    def do_POST(self):
        content_length = int(self.headers['Content-Length'])
        post_data = self.rfile.read(content_length).decode('utf-8')

        self.send_response(200)
        self.send_header('Content-Type', 'text/plain')
        self.end_headers()
        
        response = 'recv: ' + post_data
        
        self.wfile.write(response.encode('utf-8'))

def run():
    server_address = ('', 8000)
    httpd = HTTPServer(server_address, SimpleHTTPRequestHandler)
    print('Starting server on port 8000...')
    httpd.serve_forever()

run()
