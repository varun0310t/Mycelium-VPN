//go:build linux
// +build linux

package main

import (
	"fmt"
	"net"
	"syscall"
	"time"
)

func main() {
	fmt.Println("🔥 Raw Socket Test...")

	// Create raw socket
	socket := createRawSocket()
	if socket < 0 {
		fmt.Println("❌ Failed to create raw socket")
		return
	}
	defer closeRawSocket(socket)

	// Create test packet
	packet := createTestPacket("172.30.66.2", "8.8.8.8")
	if packet == nil {
		fmt.Println("❌ Failed to create packet")
		return
	}

	// Send packet
	fmt.Println("📤 Sending packet...")
	if err := sendPacket(socket, packet, "8.8.8.8"); err != nil {
		fmt.Printf("❌ Send failed: %v\n", err)
		return
	}
	if err := sendPacket(socket, packet, "8.8.8.8"); err != nil {
		fmt.Printf("❌ Send failed: %v\n", err)
		return
	}
	if err := sendPacket(socket, packet, "8.8.8.8"); err != nil {
		fmt.Printf("❌ Send failed: %v\n", err)
		return
	}

	// Listen for responses
	fmt.Println("👂 Listening for packets...")
	listenForPackets(socket, 10*time.Second)
}

// Create raw socket for sending/receiving IP packets
func createRawSocket() int {
	fmt.Println("🔧 Creating raw socket...")

	// Create raw socket for IP packets
	socket, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, syscall.IPPROTO_RAW)
	if err != nil {
		fmt.Printf("❌ Failed to create socket: %v\n", err)
		return -1
	}

	// Enable IP_HDRINCL to include IP header in packets we send
	one := 1
	err = syscall.SetsockoptInt(socket, syscall.IPPROTO_IP, syscall.IP_HDRINCL, one)
	if err != nil {
		fmt.Printf("❌ Failed to set IP_HDRINCL: %v\n", err)
		syscall.Close(socket)
		return -1
	}

	fmt.Printf("✅ Raw socket created (fd: %d)\n", socket)
	return socket
}

// Send packet using raw socket
func sendPacket(socket int, packet []byte, destIP string) error {
	fmt.Printf("📦 Sending %d bytes to %s\n", len(packet), destIP)

	// Parse destination IP
	ip := net.ParseIP(destIP)
	if ip == nil {
		return fmt.Errorf("invalid IP address: %s", destIP)
	}
	ip = ip.To4()
	if ip == nil {
		return fmt.Errorf("not IPv4 address: %s", destIP)
	}

	// Create sockaddr for destination
	addr := &syscall.SockaddrInet4{
		Port: 0, // Raw sockets don't use ports
		Addr: [4]byte{ip[0], ip[1], ip[2], ip[3]},
	}

	// Send packet
	err := syscall.Sendto(socket, packet, 0, addr)
	if err != nil {
		return fmt.Errorf("sendto failed: %v", err)
	}

	fmt.Printf("✅ Packet sent successfully (%d bytes)\n", len(packet))
	return nil
}

// Listen for incoming packets
func listenForPackets(socket int, timeout time.Duration) {
	fmt.Printf("👂 Listening for packets (timeout: %v)...\n", timeout)

	// Create capture socket for receiving (separate from send socket)
	captureSocket, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, int(htons(syscall.ETH_P_IP)))
	if err != nil {
		fmt.Printf("❌ Failed to create capture socket: %v\n", err)
		return
	}
	defer syscall.Close(captureSocket)

	buffer := make([]byte, 4096)
	deadline := time.Now().Add(timeout)
	packetCount := 0

	for time.Now().Before(deadline) {
		// Set read timeout
		tv := syscall.Timeval{
			Sec:  1, // 1 second timeout
			Usec: 0,
		}
		syscall.SetsockoptTimeval(captureSocket, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &tv)

		// Read packet
		n, _, err := syscall.Recvfrom(captureSocket, buffer, 0)
		if err != nil {
			// Check if it's timeout
			if errno, ok := err.(syscall.Errno); ok && errno == syscall.EAGAIN {
				continue
			}
			fmt.Printf("⚠️ Receive error: %v\n", err)
			continue
		}

		packetCount++
		ipPacket := buffer[14:n]
		analyzePacket(packetCount, ipPacket)

	}

	fmt.Printf("📊 Total packets received: %d\n", packetCount)
}

// Analyze received packet
func analyzePacket(count int, packet []byte) {
	if len(packet) < 20 {
		fmt.Printf("📥 Packet #%d: Too short (%d bytes)\n", count, len(packet))
		return
	}

	// Parse IP header
	version := packet[0] >> 4
	if version != 4 {
		fmt.Printf("📥 Packet #%d: Not IPv4 (version %d)\n", count, version)
		return
	}

	protocol := packet[9]
	sourceIP := net.IPv4(packet[12], packet[13], packet[14], packet[15])
	destIP := net.IPv4(packet[16], packet[17], packet[18], packet[19])

	fmt.Printf("📥 Packet #%d: %s → %s (proto %d, %d bytes)\n",
		count, sourceIP, destIP, protocol, len(packet))

	// Check for ICMP packets
	if protocol == 1 && len(packet) > 20 {
		icmpType := packet[20]
		icmpCode := packet[21]
		fmt.Printf("   ICMP Type: %d, Code: %d\n", icmpType, icmpCode)

		if icmpType == 0 {
			fmt.Printf("   🎯 ICMP Echo Reply!\n")
		} else if icmpType == 8 {
			fmt.Printf("   📤 ICMP Echo Request\n")
		}
	}
}

// Create test ICMP packet
func createTestPacket(sourceIP, destIP string) []byte {
	fmt.Printf("🔨 Creating test packet: %s → %s\n", sourceIP, destIP)

	srcIP := net.ParseIP(sourceIP).To4()
	dstIP := net.ParseIP(destIP).To4()
	if srcIP == nil || dstIP == nil {
		fmt.Println("❌ Invalid IP addresses")
		return nil
	}

	// Create 60-byte packet (20 IP + 8 ICMP + 32 data)
	packet := make([]byte, 60)

	// IP Header (20 bytes)
	packet[0] = 0x45  // Version (4) + IHL (5)
	packet[1] = 0x00  // Type of Service
	packet[2] = 0x00  // Total Length (high)
	packet[3] = 0x3C  // Total Length (low) = 60
	packet[4] = 0x12  // Identification (high)
	packet[5] = 0x34  // Identification (low)
	packet[6] = 0x00  // Flags + Fragment Offset (high)
	packet[7] = 0x00  // Fragment Offset (low)
	packet[8] = 0x40  // TTL = 64
	packet[9] = 0x01  // Protocol = ICMP
	packet[10] = 0x00 // Header Checksum (will calculate)
	packet[11] = 0x00 // Header Checksum (will calculate)

	// Source IP
	copy(packet[12:16], srcIP)

	// Destination IP
	copy(packet[16:20], dstIP)

	// Calculate IP header checksum
	ipChecksum := calculateChecksum(packet[:20])
	packet[10] = byte(ipChecksum >> 8)
	packet[11] = byte(ipChecksum & 0xFF)

	// ICMP Header (8 bytes)
	packet[20] = 0x08 // ICMP Type = Echo Request
	packet[21] = 0x00 // ICMP Code = 0
	packet[22] = 0x00 // ICMP Checksum (will calculate)
	packet[23] = 0x00 // ICMP Checksum (will calculate)
	packet[24] = 0x12 // Identifier (high)
	packet[25] = 0x34 // Identifier (low)
	packet[26] = 0x00 // Sequence Number (high)
	packet[27] = 0x01 // Sequence Number (low)

	// ICMP Data (32 bytes)
	for i := 28; i < 60; i++ {
		packet[i] = byte(i - 28)
	}

	// Calculate ICMP checksum
	icmpChecksum := calculateChecksum(packet[20:])
	packet[22] = byte(icmpChecksum >> 8)
	packet[23] = byte(icmpChecksum & 0xFF)

	fmt.Printf("✅ Created %d-byte ICMP packet\n", len(packet))
	return packet
}

// Calculate IP/ICMP checksum
func calculateChecksum(data []byte) uint16 {
	var sum uint32

	// Sum all 16-bit words
	for i := 0; i < len(data)-1; i += 2 {
		sum += uint32(data[i])<<8 + uint32(data[i+1])
	}

	// Add odd byte if present
	if len(data)%2 == 1 {
		sum += uint32(data[len(data)-1]) << 8
	}

	// Add carry bits
	for sum>>16 != 0 {
		sum = (sum & 0xFFFF) + (sum >> 16)
	}

	// Return one's complement
	return uint16(^sum)
}

// Close raw socket
func closeRawSocket(socket int) {
	if socket >= 0 {
		syscall.Close(socket)
		fmt.Printf("🧹 Closed raw socket (fd: %d)\n", socket)
	}
}

// Helper function to convert host to network byte order
func htons(i uint16) uint16 {
	return (i<<8)&0xff00 | i>>8
}
