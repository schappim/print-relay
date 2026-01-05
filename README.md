# PrintRelay

A cloud-based print relay system that enables remote printing to any printer connected to your local network. PrintRelay consists of a cloud server and lightweight clients that connect printers to the cloud via WebSocket.

## Architecture

```
                                    ┌─────────────────┐
                                    │   Cloud Server  │
                                    │   (PrintRelay)  │
                                    │                 │
                                    │  - REST API     │
                                    │  - WebSocket    │
                                    │  - Virtual PDF  │
                                    └────────┬────────┘
                                             │
                        ┌────────────────────┼────────────────────┐
                        │                    │                    │
                        ▼                    ▼                    ▼
               ┌─────────────┐      ┌─────────────┐      ┌─────────────┐
               │   Client    │      │   Client    │      │   Client    │
               │  (Office)   │      │   (Home)    │      │  (Branch)   │
               │             │      │             │      │             │
               │ ┌─────────┐ │      │ ┌─────────┐ │      │ ┌─────────┐ │
               │ │ Printer │ │      │ │ Printer │ │      │ │ Printer │ │
               │ └─────────┘ │      │ └─────────┘ │      │ └─────────┘ │
               └─────────────┘      └─────────────┘      └─────────────┘
```

## Features

- **Cloud-based print management**: Access printers from anywhere via REST API
- **WebSocket connectivity**: Real-time bidirectional communication between server and clients
- **Virtual PDF printer**: Save print jobs as PDF files (both server-side and client-side)
- **CUPS integration**: Full macOS/Linux printer support via CUPS
- **PrintNode-compatible API**: Drop-in replacement for PrintNode API
- **Print job tracking**: Real-time status updates for all print jobs
- **Multiple content types**: Support for PDF (base64/URL), raw data
- **Print options**: Copies, duplex, color, paper size, DPI, and more

## Quick Start

### Prerequisites

- Go 1.19 or later (if building from source)
- macOS or Linux (for client with CUPS)
- (Optional) Caddy or nginx for SSL termination

### Install via Homebrew (macOS)

```bash
# Add the tap
brew tap schappim/printrelay

# Install the server (for running on your cloud/server)
brew install printrelay-server

# Install the client (for machines with printers)
brew install printrelay-client
```

### Install via Script (macOS & Linux)

```bash
# Install both server and client
curl -sSL https://raw.githubusercontent.com/schappim/print-relay/main/install.sh | bash

# Install only the client
curl -sSL https://raw.githubusercontent.com/schappim/print-relay/main/install.sh | bash -s -- --client-only

# Install only the server
curl -sSL https://raw.githubusercontent.com/schappim/print-relay/main/install.sh | bash -s -- --server-only
```

### Install via APT (Debian/Ubuntu)

```bash
# Add the GPG key
curl -fsSL https://schappim.github.io/printrelay-apt/gpg.key | sudo gpg --dearmor -o /usr/share/keyrings/printrelay.gpg

# Add the repository
echo "deb [signed-by=/usr/share/keyrings/printrelay.gpg] https://schappim.github.io/printrelay-apt stable main" | sudo tee /etc/apt/sources.list.d/printrelay.list

# Update and install
sudo apt update
sudo apt install printrelay-server printrelay-client
```

### Build from Source

```bash
# Clone the repository
git clone https://github.com/schappim/print-relay.git
cd print-relay

# Build server
go build -o printrelay-server ./cmd/server

# Build client
go build -o printrelay-client ./cmd/client
```

---

## SDKs & Client Libraries

Official client libraries for integrating PrintRelay into your applications:

### Ruby

The [print_relay](https://github.com/schappim/print_relay_ruby) gem provides a modern, ergonomic Ruby interface for PrintRelay:

```ruby
gem 'print_relay'
```

```ruby
require 'print_relay'

# Configure for your PrintRelay server
PrintRelay.configure do |config|
  config.api_key = 'your-api-key'
  config.use_print_relay!('https://your-server.com')
end

# Print a PDF from URL
PrintRelay.print(
  printer_id: 1,
  url: 'https://example.com/invoice.pdf',
  title: 'Invoice #123'
)

# Print a local file
printer = PrintRelay::Printer.find(1)
printer.print_file('/path/to/document.pdf', copies: 2)
```

Features:
- Ergonomic printing from URLs, files, binary data, base64, or IO objects
- Full API coverage (computers, printers, print jobs, scales, webhooks)
- Also compatible with PrintNode cloud API
- Zero runtime dependencies

---

## Server Setup

### 1. Generate Authentication Keys

Generate secure random keys for API and client authentication:

```bash
# Generate API key (for HTTP API access)
openssl rand -hex 32
# Example output: a1b2c3d4e5f6...

# Generate client key (for WebSocket clients)
openssl rand -hex 32
# Example output: f6e5d4c3b2a1...
```

### 2. Run the Server

```bash
./printrelay-server \
  -port 8080 \
  -api-key "YOUR_API_KEY" \
  -client-key "YOUR_CLIENT_KEY" \
  -data-dir ./data \
  -verbose
```

#### Server Options

| Flag | Default | Description |
|------|---------|-------------|
| `-port` | `8080` | HTTP/WebSocket port |
| `-api-key` | (required) | API key for HTTP authentication (comma-separated for multiple) |
| `-client-key` | (required) | Key for client WebSocket authentication |
| `-data-dir` | `./data` | Directory for persistent data and PDF output |
| `-verbose` | `false` | Enable verbose logging |

### 3. Deploy with SSL (Recommended)

For production, use Caddy for automatic HTTPS:

```bash
# Install Caddy
apt install caddy

# Configure Caddy (/etc/caddy/Caddyfile)
your-domain.com {
    reverse_proxy localhost:8080
}

# Reload Caddy
systemctl reload caddy
```

### 4. Run as a Service (systemd)

Create `/etc/systemd/system/printrelay.service`:

```ini
[Unit]
Description=PrintRelay Server
After=network.target

[Service]
Type=simple
WorkingDirectory=/opt/printrelay
ExecStart=/opt/printrelay/printrelay-server \
  -port 8080 \
  -api-key "YOUR_API_KEY" \
  -client-key "YOUR_CLIENT_KEY" \
  -data-dir /opt/printrelay/data \
  -verbose
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Then enable and start:

```bash
systemctl daemon-reload
systemctl enable printrelay
systemctl start printrelay
```

---

## Client Setup

### 1. Build the Client

```bash
go build -o printrelay-client ./cmd/client
```

### 2. Run the Client

```bash
./printrelay-client \
  -server "wss://your-server.com/ws" \
  -key "YOUR_CLIENT_KEY" \
  -data-dir ./data \
  -verbose
```

#### Client Options

| Flag | Default | Description |
|------|---------|-------------|
| `-server` | `http://localhost:8080` | Server URL (http/https/ws/wss) |
| `-key` | (required) | Client authentication key |
| `-data-dir` | `./data` | Directory for temp files and local PDF output |
| `-verbose` | `false` | Enable verbose logging |

### 3. Run as a Service (macOS launchd)

Create `~/Library/LaunchAgents/com.printrelay.client.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.printrelay.client</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/printrelay-client</string>
        <string>-server</string>
        <string>wss://your-server.com/ws</string>
        <string>-key</string>
        <string>YOUR_CLIENT_KEY</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
</dict>
</plist>
```

Load the service:

```bash
launchctl load ~/Library/LaunchAgents/com.printrelay.client.plist
```

### 4. Run as a Service (Linux systemd)

Create `/etc/systemd/system/printrelay-client.service`:

```ini
[Unit]
Description=PrintRelay Client
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/printrelay-client \
  -server "wss://your-server.com/ws" \
  -key "YOUR_CLIENT_KEY" \
  -data-dir /var/lib/printrelay
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

---

## API Reference

All API endpoints require HTTP Basic Authentication with the API key as username (password can be empty).

### Authentication

```bash
curl -u "YOUR_API_KEY:" https://your-server.com/ping
```

### Endpoints

#### Health Check

```
GET /ping
```

Returns `"pong"` if server is running.

#### Account Info

```
GET /whoami
```

Returns account information and connected computers.

#### List Computers

```
GET /computers
```

Returns all registered computers.

#### List Printers

```
GET /printers
GET /printers/{id}
GET /computers/{id}/printers
```

Returns available printers.

#### Create Print Job

```
POST /printjobs
Content-Type: application/json

{
  "printerId": 1,
  "title": "My Document",
  "contentType": "pdf_uri",
  "content": "https://example.com/document.pdf",
  "source": "My App",
  "qty": 1,
  "options": {
    "copies": 1,
    "color": true,
    "duplex": "long-edge",
    "paper": "A4"
  }
}
```

**Content Types:**
- `pdf_base64` - Base64-encoded PDF
- `pdf_uri` - URL to PDF file
- `raw_base64` - Base64-encoded raw data
- `raw_uri` - URL to raw data

**Options:**
- `copies` - Number of copies
- `color` - Color printing (true/false)
- `duplex` - Duplex mode: `long-edge`, `short-edge`, `none`
- `paper` - Paper size: `Letter`, `Legal`, `A4`, `A3`, `A5`
- `bin` - Paper tray
- `dpi` - Resolution: `300dpi`, `600dpi`, `1200dpi`
- `pages` - Page range: `1-5`, `1,3,5`
- `fitToPage` - Scale to fit page (true/false)
- `nup` - Pages per sheet: 1, 2, 4, 6, 9, 16

#### Get Print Job Status

```
GET /printjobs/{id}
```

**Job States:**
- `new` - Job created
- `sent_to_client` - Sent to client
- `received` - Received by client
- `downloading` - Downloading content
- `queued` - Queued at printer
- `printing` - Currently printing
- `done` - Completed
- `error` - Failed
- `cancelled` - Cancelled

#### Delete Print Jobs

```
DELETE /printjobs/{id}
DELETE /printjobs  # Delete all
```

---

## Virtual PDF Printers

PrintRelay includes virtual PDF printers that save jobs as files instead of printing:

### Server-Side Virtual Printer

- **Printer ID**: `-1`
- **Name**: `PDF_Virtual_Printer`
- Saves PDFs to `{data-dir}/pdf_output/` on the server

### Client-Side Virtual Printer

- **Name**: `PDF_Virtual_Printer_Local`
- Saves PDFs to `{data-dir}/pdf_output/` on the client machine

---

## Examples

### Print a PDF from URL

```bash
curl -u "API_KEY:" https://your-server.com/printjobs \
  -H "Content-Type: application/json" \
  -d '{
    "printerId": 1,
    "title": "Invoice",
    "contentType": "pdf_uri",
    "content": "https://example.com/invoice.pdf"
  }'
```

### Print a Base64-encoded PDF

```bash
curl -u "API_KEY:" https://your-server.com/printjobs \
  -H "Content-Type: application/json" \
  -d '{
    "printerId": 1,
    "title": "Report",
    "contentType": "pdf_base64",
    "content": "JVBERi0xLjQK..."
  }'
```

### Print with Options

```bash
curl -u "API_KEY:" https://your-server.com/printjobs \
  -H "Content-Type: application/json" \
  -d '{
    "printerId": 1,
    "title": "Document",
    "contentType": "pdf_uri",
    "content": "https://example.com/doc.pdf",
    "qty": 1,
    "options": {
      "copies": 2,
      "duplex": "long-edge",
      "paper": "A4",
      "color": false
    }
  }'
```

### Save to Virtual Printer

```bash
# Server-side virtual printer
curl -u "API_KEY:" https://your-server.com/printjobs \
  -H "Content-Type: application/json" \
  -d '{
    "printerId": -1,
    "title": "Saved Document",
    "contentType": "pdf_uri",
    "content": "https://example.com/doc.pdf"
  }'
```

---

## Project Structure

```
print-relay/
├── cmd/
│   ├── server/         # Server entry point
│   │   └── main.go
│   └── client/         # Client entry point
│       └── main.go
├── cloudserver/        # Server implementation
│   ├── server.go       # HTTP/WebSocket handlers
│   ├── hub.go          # WebSocket connection hub
│   └── store.go        # Data persistence
├── client/             # Client implementation
│   └── client.go       # CUPS printing, WebSocket client
├── protocol/           # Shared protocol definitions
│   └── protocol.go     # Message types, constants
├── go.mod
├── go.sum
└── README.md
```

---

## Troubleshooting

### Client disconnects frequently

Ensure the server and client can maintain a stable WebSocket connection. Check firewall settings and network stability.

### Print jobs fail with "Computer not connected"

The client for that printer is not connected. Verify the client is running and can reach the server.

### PDF download fails

Some servers block requests without proper User-Agent headers. PrintRelay includes browser-like headers, but very restrictive servers may still block requests.

### CUPS printing fails

Ensure CUPS is properly configured on the client machine:

```bash
# Check CUPS status
lpstat -p -d

# Test local printing
lp -d "Printer_Name" /path/to/test.pdf
```

---

## License

Copyright (c) 2026, Ninja AI Labs Pty Ltd.

This software is provided under a permissive license with an anti-SaaS competition clause. You may use, modify, and distribute this software freely, but you may not offer it as a competing hosted service. See [LICENSE](LICENSE) for full terms.

## Contributing

Contributions are welcome! Please open an issue or submit a pull request.
