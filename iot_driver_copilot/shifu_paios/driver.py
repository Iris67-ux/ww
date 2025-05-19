import os
import json
from http.server import BaseHTTPRequestHandler, HTTPServer
from urllib.parse import urlparse, parse_qs
from threading import Thread

# Mocked device telemetry and state (in practice, this would be realtime, from device APIs)
import random
import time

# --- Environment Variables ---
DEVICE_NAME = os.getenv('DEVICE_NAME', 'Shifu PAIOS')
DEVICE_MODEL = os.getenv('DEVICE_MODEL', 'PAIOS')
DEVICE_MANUFACTURER = os.getenv('DEVICE_MANUFACTURER', 'Shifu')
DEVICE_TYPE = os.getenv('DEVICE_TYPE', 'Physical AI Operating System, IoT Edge Device Platform')
SERVER_HOST = os.getenv('SERVER_HOST', '0.0.0.0')
SERVER_PORT = int(os.getenv('SERVER_PORT', '8000'))

# Simulated telemetry data generator
telemetry_data = {
    "cpu_usage": 13.7,
    "memory_usage": 42,
    "temperature": 37.8,
    "device_state": "active",
    "uptime": 0,
    "timestamp": time.time()
}

def update_telemetry():
    global telemetry_data
    while True:
        telemetry_data = {
            "cpu_usage": round(random.uniform(10, 80), 2),
            "memory_usage": random.randint(30, 85),
            "temperature": round(random.uniform(36, 55), 2),
            "device_state": random.choice(["active", "idle", "maintenance"]),
            "uptime": int(time.monotonic()),
            "timestamp": time.time()
        }
        time.sleep(2)

class ShifuPAIOSHandler(BaseHTTPRequestHandler):
    def _set_headers(self, status=200, content_type="application/json"):
        self.send_response(status)
        self.send_header('Content-type', content_type)
        self.end_headers()

    def do_GET(self):
        parsed = urlparse(self.path)
        if parsed.path == '/telemetry':
            self._set_headers()
            self.wfile.write(json.dumps(telemetry_data).encode())
        elif parsed.path == '/info':
            self._set_headers()
            info = {
                "device_name": DEVICE_NAME,
                "device_model": DEVICE_MODEL,
                "manufacturer": DEVICE_MANUFACTURER,
                "device_type": DEVICE_TYPE
            }
            self.wfile.write(json.dumps(info).encode())
        else:
            self._set_headers(404)
            self.wfile.write(json.dumps({"error": "Not found"}).encode())

    def do_POST(self):
        parsed = urlparse(self.path)
        if parsed.path == '/ctrl':
            content_length = int(self.headers.get('Content-Length', 0))
            post_body = self.rfile.read(content_length)
            try:
                command = json.loads(post_body.decode())
            except Exception:
                self._set_headers(400)
                self.wfile.write(json.dumps({"error": "Invalid JSON"}).encode())
                return

            # Simulate command handling
            cmd_type = command.get('type')
            response = {"status": "success", "command_received": command}
            if cmd_type == 'reboot':
                response['message'] = "Device will reboot."
            elif cmd_type == 'update':
                response['message'] = "OTA update initiated."
            elif cmd_type == 'set_state':
                telemetry_data['device_state'] = command.get('state', telemetry_data['device_state'])
                response['message'] = f"Device state set to {telemetry_data['device_state']}."
            else:
                response['message'] = "Generic command executed."

            self._set_headers()
            self.wfile.write(json.dumps(response).encode())
        else:
            self._set_headers(404)
            self.wfile.write(json.dumps({"error": "Not found"}).encode())

    def log_message(self, format, *args):
        # Suppress default logging
        pass

if __name__ == '__main__':
    telemetry_thread = Thread(target=update_telemetry, daemon=True)
    telemetry_thread.start()
    server = HTTPServer((SERVER_HOST, SERVER_PORT), ShifuPAIOSHandler)
    print(f"Shifu PAIOS Driver HTTP Server running at http://{SERVER_HOST}:{SERVER_PORT}")
    server.serve_forever()
