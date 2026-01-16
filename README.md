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

## How to Use

### Server Setup (Docker - Linux/Windows/Mac)

The VPN server runs in a Docker container and works on any platform that supports Docker.

1. **Configure the server**  
   Place your server configuration at [./config/ServerConfig.json](./config/ServerConfig.json) or prepare to mount it into the container.

2. **Build and run the server**
   ````bash
       docker-compose up --build
    
   ````

### Client Setup ( Linux and Windows)

## Linux

1. **git Clone the repo**

2. **Install the client**
   Run the installation script with root privileges:
   ```bash
   sudo ./InstallClient.sh
   ```
3. **Connect to the VPN**
   ```bash
   mycelium connect --server <SERVER_IP> --port <PORT_NUM> --key <AUTH_KEY>
   ```
   Or use the config file:
   ```bash
   mycelium connect
   ```
4. **Disconnect**
   ```bash
   mycelium disconnect
   ```

## Windows

1. **git Clone the repo**

2. **Open Windows PowerShell With admin Priveledge**
   This cli need admin priveledge for installation and During Connecting and Disconnection process of the vpn
   
3. **Install the client**
   Run the installation script with root privileges:
   ```bash
   .\InstallWindowsClient.ps1
   ```
4. **Connect to the VPN**
   ```bash
   mycelium connect --server <SERVER_IP> --port <PORT_NUM> --key <AUTH_KEY>
   ```
   Or use the config file:
   ```bash
   mycelium connect
   ```
5. **Disconnect**
   ```bash
   mycelium disconnect
   ```
6. **Help**
   ```bash
   mycelium help
   ```
