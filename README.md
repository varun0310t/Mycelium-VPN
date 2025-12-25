# VPN

A personal, lightweight self-hostable VPN (server + client) implemented in Go.

## Key points
- Functional end-to-end VPN with TUN interface, NAT, routing and client authentication.
- Uses DTLS for secure transport between client and server.
- Low latency in tests (responsive).
- Bandwidth issue investigated and fixed — observed speeds after fixes: ~70 Mbit/s download / ~28 Mbit/s upload (results may vary by network). Previously observed ~200 KB/s.
- Originally started as a learning side project — now a functional lightweight VPN suited for low‑bandwidth and many typical personal use cases.

## Features
- TUN interface creation and configuration
- NAT masquerading and IP forwarding for client internet access
- DTLS-secured transport
- Simple client/server protocol with keep-alive and authentication
- Dockerfile and docker-compose for quick deployment
- Config file support (place config in `./config/ServerConfig.json` or mount into container)

## Current limitations
- Performance depends on host, Docker networking mode and underlying network; results will vary.
- Not hardened for large-scale production by default (TLS/DTLS cert management optional).
- Currently only supports Linux (can be ported with OS-specific changes).

## Quick start (development)
1. Build and run:
    ```bash
    docker-compose up --build
    ```
2. Place server config at `./config/ServerConfig.json` or mount it into `/app/ServerConfig.json` in the container.
3. Use the included client (container or native) — requires root to create TUN device.

