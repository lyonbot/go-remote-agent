"""
curl -X POST http://127.0.0.1:8080/ -H "Host: foobar.com" -d 'hello world' -v
"""
import asyncio
import websockets
import logging
import pathlib
import http
from http.server import SimpleHTTPRequestHandler
import socketserver
import threading

logging.basicConfig(
    format='%(asctime)s %(levelname)s: %(message)s',
    level=logging.INFO
)

# HTML content for the homepage
HTML = """
<!DOCTYPE html>
<html>
<head>
    <title>WebSocket Echo Test</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            max-width: 800px;
            margin: 0 auto;
            padding: 20px;
            background-color: #f5f5f5;
        }
        .container {
            background-color: white;
            padding: 20px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
        }
        #messageLog {
            width: 100%;
            height: 200px;
            margin: 10px 0;
            padding: 10px;
            border: 1px solid #ddd;
            border-radius: 4px;
            overflow-y: auto;
            background-color: #fff;
        }
        input[type="text"] {
            width: 70%;
            padding: 8px;
            margin-right: 10px;
            border: 1px solid #ddd;
            border-radius: 4px;
        }
        button {
            padding: 8px 15px;
            background-color: #4CAF50;
            color: white;
            border: none;
            border-radius: 4px;
            cursor: pointer;
        }
        button:hover {
            background-color: #45a049;
        }
        .status {
            margin: 10px 0;
            padding: 10px;
            border-radius: 4px;
        }
        .connected {
            background-color: #dff0d8;
            color: #3c763d;
        }
        .disconnected {
            background-color: #f2dede;
            color: #a94442;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>WebSocket Echo Test</h1>
        <div id="connectionStatus" class="status disconnected">Disconnected</div>
        <div>
            <input type="text" id="messageInput" placeholder="Enter a message...">
            <button onclick="sendMessage()">Send</button>
        </div>
        <div id="messageLog"></div>
    </div>

    <script>
        let socket = null;
        const messageLog = document.getElementById('messageLog');
        const connectionStatus = document.getElementById('connectionStatus');
        const messageInput = document.getElementById('messageInput');

        function connect() {
            socket = new WebSocket('ws://localhost:8765');

            socket.onopen = function(e) {
                connectionStatus.textContent = 'Connected';
                connectionStatus.className = 'status connected';
                log('Connection established');
            };

            socket.onmessage = function(event) {
                log('Received: ' + event.data);
            };

            socket.onclose = function(event) {
                connectionStatus.textContent = 'Disconnected';
                connectionStatus.className = 'status disconnected';
                log('Connection closed. Reconnecting in 5 seconds...');
                setTimeout(connect, 5000);
            };

            socket.onerror = function(error) {
                log('WebSocket error: ' + error.message);
            };
        }

        function sendMessage() {
            const message = messageInput.value;
            if (message && socket && socket.readyState === WebSocket.OPEN) {
                socket.send(message);
                log('Sent: ' + message);
                messageInput.value = '';
            }
        }

        function log(message) {
            const div = document.createElement('div');
            div.textContent = message;
            messageLog.appendChild(div);
            messageLog.scrollTop = messageLog.scrollHeight;
        }

        messageInput.addEventListener('keypress', function(e) {
            if (e.key === 'Enter') {
                sendMessage();
            }
        });

        // Initial connection
        connect();
    </script>
</body>
</html>
"""

async def handle_http_request(connection, request):
    """Handle HTTP request and return the homepage"""

    if request.headers.get("Upgrade", "").lower() == "websocket":
        return
    
    if request.path == "/":
        resp = connection.respond(http.HTTPStatus.OK, HTML)
        resp.headers["Content-Type"] = "text/html"
        resp.headers["Content-Length"] = str(len(HTML))
        return resp
    return connection.respond(http.HTTPStatus.NOT_FOUND, '404')

async def echo(websocket):
    """
    Echo handler for WebSocket connections.
    Receives messages and sends them back to the client.
    """
    try:
        async for message in websocket:
            logging.info(f"Received message: {message}")
            await websocket.send(message)
            logging.info(f"Sent message: {message}")
    except websockets.exceptions.ConnectionClosed:
        logging.info("Client connection closed")
    except Exception as e:
        logging.error(f"Error handling connection: {e}")

async def main():
    """
    Main server function that starts both the WebSocket server and HTTP server
    """
    # Start the WebSocket server with HTTP server capability
    async with websockets.serve(
        echo,
        "localhost",
        8765,
        process_request=handle_http_request
    ):
        logging.info("Server started:")
        logging.info("- WebSocket endpoint: ws://localhost:8765")
        logging.info("- Homepage: http://localhost:8765")
        # Run forever
        await asyncio.Future()

if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        logging.info("Server stopped by user")
    except Exception as e:
        logging.error(f"Server error: {e}")