#!/bin/bash
set -e

echo "🐳 Starting VPN Packet Forwarding Test in Docker..."

# Enable IP forwarding
echo "📡 Enabling IP forwarding..."
echo 1 > /proc/sys/net/ipv4/ip_forward

# Create TUN device (if needed)
echo "🔧 Setting up network interfaces..."
if [ ! -c /dev/net/tun ]; then
    mkdir -p /dev/net
    mknod /dev/net/tun c 10 200
    chmod 600 /dev/net/tun
fi

# Show current network setup
echo "🌐 Current network interfaces:"
ip addr show

echo "🛣️ Current routing table:"
ip route show

# Set up basic NAT rules for internet access
echo "🔥 Setting up iptables NAT rules..."
iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE
iptables -A FORWARD -i tun+ -o eth0 -j ACCEPT
iptables -A FORWARD -i eth0 -o tun+ -m state --state RELATED,ESTABLISHED -j ACCEPT

echo "✅ Docker container ready!"
echo "🚀 Running packet forwarding test..."

# Run the Go application
exec ./packetForwarding