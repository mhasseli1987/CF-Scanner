# Cloudflare Clean IP Scanner

A fast, multi-threaded scanner that finds clean Cloudflare IPs with low latency.

## Features

- Scans all official Cloudflare IPv4 ranges randomly
- 128 concurrent workers for high-speed scanning
- Filters IPs with latency under 200ms (TCP handshake on port 443)
- Outputs 50 clean IPs sorted by latency
- Saves results to a plain text file (IPs only, one per line)

## Usage

1. Download `CF-Scanner.exe` from the [Releases](../../releases) page
2. Double-click to run
3. Wait for the scan to complete
4. A `clean_ips_<date>.txt` file will be created in the same directory

## Output Format

The output file contains only IP addresses, one per line:

```
104.18.32.47
172.67.182.31
104.21.56.78
...
```

## Build from Source

```bash
# Requires Go 1.22+
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o CF-Scanner.exe main.go
```

## Cloudflare IP Ranges

The scanner covers all official Cloudflare IPv4 ranges as listed on [cloudflare.com/ips](https://www.cloudflare.com/ips/).

## Note

- Windows Defender may flag the exe as unknown — click "Run anyway"
- Run without VPN for accurate latency results
- Results vary by network and time of day
