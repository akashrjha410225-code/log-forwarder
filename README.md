# Enterprise Log Forwarder Agent

A lightweight, highly efficient, cross-platform log forwarding agent written in Go. This agent tails log files in real-time, applies regex-based noise reduction, batches events, and ships them as structured JSON to any HTTP/HTTPS endpoint (like a SIEM or log analytics platform).

## 🚀 Features

* **High Performance:** Compiled into a single, standalone binary with zero external runtime dependencies.
* **Multi-File Globbing:** Monitor specific files or entire directories using wildcards (e.g., `/var/log/*.log`).
* **Noise Reduction (Pre-filtering):** Drop useless or noisy logs at the source using Regular Expressions before they consume network bandwidth.
* **Intelligent Batching:** Groups logs in memory and flushes them based on size or time intervals to optimize API ingestion.
* **CPU/Resource Throttling:** Built-in rate limiter prevents the agent from spiking host CPU when reading massive log files.
* **Graceful Shutdown:** Intercepts system shutdown signals to flush remaining memory buffers, ensuring zero data loss during restarts.

## ⚙️ Configuration

The agent is controlled via a `config.yaml` file that must be placed in the same directory as the executable. 

Create a `config.yaml` file with the following structure:

```yaml
destination_url: "https://your-siem-ip/api/ingest"
auth_key: "your-api-key"

# Track specific files or entire directories
log_paths:
  - "/var/log/*.log"
  - "/var/log/nginx/access.log"
  # - "C:\\inetpub\\logs\\*.log" # Windows example

# Drop logs that match these regex patterns BEFORE sending
drop_rules:
  - "(?i)debug" 

# Performance & Batching
batch_size: 500
flush_interval_seconds: 5
max_mb_per_second: 10

🛠️ Build Instructions
Ensure you have Go installed on your machine.

Clone the repository:

Bash
git clone [https://github.com/akashrjha410225-code/log-forwarder.git](https://github.com/akashrjha410225-code/log-forwarder.git)
cd log-forwarder
Download dependencies:

Bash
go mod tidy
Compile the agent:
For Linux/macOS:

Bash
go build -o log-agent
For Windows:

Bash
GOOS=windows GOARCH=amd64 go build -o log-agent.exe
🚀 Usage
Simply run the compiled binary. It will automatically look for config.yaml in the same directory.

Bash
./log-agent
📦 JSON Payload Structure
The agent automatically wraps raw log lines into the following structured JSON array before HTTP transmission:

JSON
[
  {
    "timestamp": "2023-10-27T10:00:00Z",
    "host": "production-web-01",
    "source": "/var/log/nginx/access.log",
    "message": "192.168.1.1 - - [27/Oct/2023:10:00:00 +0000] \"GET / HTTP/1.1\" 200"
  }
]