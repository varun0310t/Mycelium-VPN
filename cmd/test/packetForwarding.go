//go:build linux
// +build linux

package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"syscall"
	"time"
	"unsafe"
)

// Linux TUN constants
const (
	IFF_TUN   = 0x0001
	IFF_NO_PI = 0x1000
	TUNSETIFF = 0x400454ca
)

type NativeTUN struct {
	file *os.File
	name string
}

func main() {
	fmt.Println("🐧 Native Linux TUN Packet Forwarding Test...")

	// Create native TUN interface
	tun := createNativeTUN("vpn0")
	if tun == nil {
		fmt.Println("❌ Failed to create TUN interface")
		return
	}
	defer tun.Close()

	// Configure the interface
	if !configureTUN(tun.name, "10.0.0.1/24") {
		return
	}

	// Setup routing
	setupRouting(tun.name)
	defer cleanupRouting(tun.name)

	time.Sleep(10 * time.Second)

	// Create and send ping packet
	pingPacket := createUDPZeroChecksumPacket("8.8.8.8")
	if pingPacket == nil {
		fmt.Println("❌ Failed to create ping packet")
		return
	}

	err := tun.WritePacket(pingPacket)
	if err != nil {
		fmt.Printf("❌ Failed to write packet: %v\n", err)
		return
	}

	fmt.Println("👂 Listening for response...")
	response := tun.ReadPackets(120 * time.Second)

	if response != nil {
		fmt.Printf("✅ SUCCESS! Received response: %d bytes\n", len(response))
		analyzeResponse(response)
	} else {
		fmt.Println("❌ No response received")
		debugNetwork()
	}
}

func createNativeTUN(ifname string) *NativeTUN {
	fmt.Println("🔧 Creating native Linux TUN interface...")

	// Ensure /dev/net/tun exists
	if err := ensureTUNDevice(); err != nil {
		fmt.Printf("❌ Failed to ensure TUN device: %v\n", err)
		return nil
	}

	// Open TUN device file
	file, err := os.OpenFile("/dev/net/tun", os.O_RDWR, 0)
	if err != nil {
		fmt.Printf("❌ Failed to open /dev/net/tun: %v\n", err)
		return nil
	}

	// Prepare interface request structure
	var ifr struct {
		name  [16]byte
		flags uint16
		_     [22]byte // padding to match C struct
	}

	// Set interface name and flags
	copy(ifr.name[:], ifname)
	ifr.flags = IFF_TUN | IFF_NO_PI // TUN interface, no packet info

	// Create TUN interface using ioctl
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		file.Fd(),
		TUNSETIFF,
		uintptr(unsafe.Pointer(&ifr)),
	)

	if errno != 0 {
		fmt.Printf("❌ ioctl TUNSETIFF failed: %v\n", errno)
		file.Close()
		return nil
	}

	// Get actual interface name (might be different from requested)
	actualName := extractStringFromBytes(ifr.name[:])
	fmt.Printf("✅ Created native TUN interface: %s\n", actualName)

	return &NativeTUN{
		file: file,
		name: actualName,
	}
}

func ensureTUNDevice() error {
	// Check if /dev/net/tun already exists
	if _, err := os.Stat("/dev/net/tun"); err == nil {
		return nil
	}

	// Create /dev/net directory if it doesn't exist
	if err := os.MkdirAll("/dev/net", 0755); err != nil {
		return fmt.Errorf("failed to create /dev/net: %v", err)
	}

	// Create TUN device node (major=10, minor=200)
	return syscall.Mknod("/dev/net/tun", syscall.S_IFCHR|0600, int((10<<8)|200))
}

func extractStringFromBytes(b []byte) string {
	for i, char := range b {
		if char == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

func (t *NativeTUN) WritePacket(packet []byte) error {
	fmt.Printf("🔍 Writing %d bytes to TUN interface %s\n", len(packet), t.name)

	n, err := t.file.Write(packet)
	if err != nil {
		return fmt.Errorf("TUN write failed: %v", err)
	}

	if n != len(packet) {
		return fmt.Errorf("partial write: wrote %d of %d bytes", n, len(packet))
	}

	fmt.Printf("✅ Successfully wrote %d bytes to TUN\n", n)
	return nil
}

func (t *NativeTUN) ReadPackets(timeout time.Duration) []byte {
	fmt.Println("📡 Starting packet capture...")

	deadline := time.Now().Add(timeout)
	buffer := make([]byte, 1500)
	packetCount := 0

	for time.Now().Before(deadline) {
		// Set read timeout
		t.file.SetReadDeadline(time.Now().Add(100 * time.Millisecond))

		n, err := t.file.Read(buffer)
		if err != nil {
			if os.IsTimeout(err) {
				continue
			}
			fmt.Printf("Read error: %v\n", err)
			continue
		}

		if n > 0 {
			packetCount++
			packet := make([]byte, n)
			copy(packet, buffer[:n])

			fmt.Printf("📥 Received packet #%d (%d bytes)\n", packetCount, n)

			if len(packet) >= 20 {
				sourceIP := net.IPv4(packet[12], packet[13], packet[14], packet[15])
				destIP := net.IPv4(packet[16], packet[17], packet[18], packet[19])
				protocol := packet[9]

				fmt.Printf("   %s → %s (protocol %d)\n", sourceIP, destIP, protocol)

				if isOurPingResponse(packet) {
					fmt.Printf("🎯 Found our ping response!\n")
					return packet
				}

				if protocol == 1 && len(packet) > 20 {
					icmpType := packet[20]
					fmt.Printf("   ICMP type: %d\n", icmpType)
				}
			}
		}
	}

	fmt.Printf("📊 Total packets received: %d\n", packetCount)
	return nil
}

func (t *NativeTUN) Close() {
	if t.file != nil {
		t.file.Close()
	}
}

func configureTUN(ifname, cidr string) bool {
	fmt.Printf("🔧 Configuring TUN interface: %s with %s\n", ifname, cidr)

	// Assign IP address
	cmd := exec.Command("ip", "addr", "add", cidr, "dev", ifname)
	if err := cmd.Run(); err != nil {
		fmt.Printf("⚠️ IP address assignment failed: %v\n", err)
		return false
	}

	// Bring interface up
	cmd = exec.Command("ip", "link", "set", ifname, "up")
	if err := cmd.Run(); err != nil {
		fmt.Printf("⚠️ Interface up failed: %v\n", err)
		return false
	}

	fmt.Printf("✅ TUN interface %s configured successfully\n", ifname)
	return true
}

func setupRouting(ifname string) {
	fmt.Println("🛣️ Setting up routing...")

	//Add route for 8.8.8.8 through TUN
	// cmd := exec.Command("ip", "route", "add", "8.8.8.8/32", "dev", ifname)
	// if err := cmd.Run(); err != nil {
	// 	fmt.Printf("⚠️ Route add failed: %v\n", err)
	// } else {
	// 	fmt.Printf("✅ Route added: 8.8.8.8 → %s\n", ifname)
	// }
	// We insert them at the top of the chains to make sure we see them.

	// Disable RPF on all interfaces
	exec.Command("sysctl", "-w", "net.ipv4.conf.all.rp_filter=0").Run()
	// Disable RPF specifically on the new vpn interface
	exec.Command("sysctl", "-w", fmt.Sprintf("net.ipv4.conf.%s.rp_filter=0", ifname)).Run()
	// Disable RPF on the main ethernet interface as well
	exec.Command("sysctl", "-w", "net.ipv4.conf.eth0.rp_filter=0").Run()

	exec.Command("iptables", "-t", "nat", "-I", "PREROUTING", "1", "-s", "10.0.0.1", "-j", "LOG", "--log-prefix", "VPN_NAT_PREROUTING: ").Run()
	exec.Command("iptables", "-I", "FORWARD", "1", "-s", "10.0.0.1", "-j", "LOG", "--log-prefix", "VPN_FILTER_FORWARD: ").Run()
	exec.Command("iptables", "-t", "nat", "-I", "POSTROUTING", "1", "-s", "10.0.0.1", "-j", "LOG", "--log-prefix", "VPN_NAT_POSTROUTING: ").Run()

	// Set up iptables rules for NAT
	fmt.Println("🔥 Setting up iptables rules...")
	// --- FIX IS HERE: Use "-I" to insert rules at the top of the chain ---
	exec.Command("iptables", "-I", "FORWARD", "1", "-i", ifname, "-j", "ACCEPT").Run()
	exec.Command("iptables", "-I", "FORWARD", "1", "-o", ifname, "-j", "ACCEPT").Run()
	exec.Command("iptables", "-t", "nat", "-A", "POSTROUTING", "-s", "10.0.0.0/24", "-j", "MASQUERADE").Run()

	// Show current routing table
	fmt.Println("📋 Current routing table:")
	cmd := exec.Command("ip", "route", "show")
	output, _ := cmd.Output()
	fmt.Printf("%s\n", string(output))
}

func cleanupRouting(ifname string) {
	fmt.Println("🧹 Cleaning up routing...")

	// Cleanup the logging rules
	exec.Command("iptables", "-t", "nat", "-D", "PREROUTING", "-s", "10.0.0.1", "-j", "LOG", "--log-prefix", "VPN_NAT_PREROUTING: ").Run()
	exec.Command("iptables", "-D", "FORWARD", "-s", "10.0.0.1", "-j", "LOG", "--log-prefix", "VPN_FILTER_FORWARD: ").Run()
	exec.Command("iptables", "-t", "nat", "-D", "POSTROUTING", "-s", "10.0.0.1", "-j", "LOG", "--log-prefix", "VPN_NAT_POSTROUTING: ").Run()
	exec.Command("ip", "route", "del", "8.8.8.8/32").Run()
	exec.Command("iptables", "-D", "FORWARD", "-i", ifname, "-j", "ACCEPT").Run()
	exec.Command("iptables", "-D", "FORWARD", "-o", ifname, "-j", "ACCEPT").Run()
	exec.Command("iptables", "-t", "nat", "-D", "POSTROUTING", "-s", "10.0.0.0/24", "-j", "MASQUERADE").Run()
}

func createPingPacket(destIP string) []byte {
	packet := make([]byte, 60)

	// IP Header (20 bytes)
	packet[0] = 0x45  // Version (4) + IHL (5)
	packet[1] = 0x00  // Type of Service
	packet[2] = 0x00  // Total Length (high)
	packet[3] = 0x3C  // Total Length (low) = 60
	packet[4] = 0x00  // Identification (high)
	packet[5] = 0x01  // Identification (low)
	packet[6] = 0x00  // Flags + Fragment Offset (high)
	packet[7] = 0x00  // Fragment Offset (low)
	packet[8] = 0x40  // TTL = 64
	packet[9] = 0x01  // Protocol = ICMP
	packet[10] = 0x00 // Header Checksum (will calculate)
	packet[11] = 0x00 // Header Checksum (will calculate)

	// Source IP: 10.0.0.1
	packet[12] = 10
	packet[13] = 0
	packet[14] = 0
	packet[15] = 1

	// Destination IP: 8.8.8.8
	destIPAddr := net.ParseIP(destIP).To4()
	if destIPAddr == nil {
		fmt.Printf("❌ Invalid destination IP: %s\n", destIP)
		return nil
	}
	copy(packet[16:20], destIPAddr)

	// Calculate IP header checksum
	checksum := calculateChecksum(packet[:20])
	packet[10] = byte(checksum >> 8)
	packet[11] = byte(checksum & 0xFF)

	// ICMP Header (8 bytes)
	packet[20] = 0x08 // ICMP Type = Echo Request
	packet[21] = 0x00 // ICMP Code = 0
	packet[22] = 0x00 // ICMP Checksum (will calculate)
	packet[23] = 0x00 // ICMP Checksum (will calculate)
	packet[24] = 0x00 // Identifier (high)
	packet[25] = 0x01 // Identifier (low)
	packet[26] = 0x00 // Sequence Number (high)
	packet[27] = 0x01 // Sequence Number (low)

	// ICMP Data
	for i := 28; i < 60; i++ {
		packet[i] = byte(i - 28)
	}

	// Calculate ICMP checksum
	icmpChecksum := calculateChecksum(packet[20:])
	packet[22] = byte(icmpChecksum >> 8)
	packet[23] = byte(icmpChecksum & 0xFF)

	fmt.Printf("📦 Created ICMP ping packet: 10.0.0.1 → %s (%d bytes)\n", destIP, len(packet))
	return packet
}
func createUDPZeroChecksumPacket(destIP string) []byte {
	// Total Packet: IP Header (20) + UDP Header (8) + Data (12) = 40 bytes
	const ipHeaderLen = 20
	const udpHeaderLen = 8
	const dataLen = 12
	const totalLen = ipHeaderLen + udpHeaderLen + dataLen

	packet := make([]byte, totalLen)

	// --- IP Header (20 bytes) ---
	packet[0] = 0x45     // Version (4) + IHL (5)
	packet[1] = 0x00     // Type of Service
	packet[2] = 0x00     // Total Length (high byte)
	packet[3] = totalLen // Total Length (low byte) = 40
	packet[4] = 0x00     // Identification (high)
	packet[5] = 0x01     // Identification (low)
	packet[6] = 0x00     // Flags + Fragment Offset
	packet[7] = 0x00     // Fragment Offset
	packet[8] = 0x40     // TTL = 64
	packet[9] = 17       // Protocol = UDP (17)
	packet[10] = 0x00    // Header Checksum (will be calculated)
	packet[11] = 0x00    // Header Checksum (will be calculated)

	// Source IP: 10.0.0.1
	packet[12] = 10
	packet[13] = 0
	packet[14] = 0
	packet[15] = 1

	// Destination IP (from function argument)
	destIPAddr := net.ParseIP(destIP).To4()
	copy(packet[16:20], destIPAddr)

	// Calculate and set the mandatory IP header checksum
	ipChecksum := calculateChecksum(packet[0:ipHeaderLen])
	packet[10] = byte(ipChecksum >> 8)
	packet[11] = byte(ipChecksum & 0xFF)

	// --- UDP Header (8 bytes) ---
	udpStart := ipHeaderLen
	// Source Port (e.g., 43210)
	packet[udpStart+0] = 0xA8
	packet[udpStart+1] = 0xCA
	// Destination Port (e.g., 53 for DNS)
	packet[udpStart+2] = 0x00
	packet[udpStart+3] = 0x35
	// UDP Length (UDP Header + Data)
	udpLen := udpHeaderLen + dataLen
	packet[udpStart+4] = byte(udpLen >> 8)
	packet[udpStart+5] = byte(udpLen & 0xFF)
	// UDP Checksum
	packet[udpStart+6] = 0x00 // The key part: set checksum to 0
	packet[udpStart+7] = 0x00 // The kernel will accept this

	// --- Data Payload (12 bytes) ---
	dataStart := ipHeaderLen + udpHeaderLen
	copy(packet[dataStart:], []byte("hello-world!"))

	fmt.Printf("📦 Created UDP packet: 10.0.0.1 → %s (%d bytes) with zero checksum\n", destIP, len(packet))
	return packet
}
func isOurPingResponse(packet []byte) bool {
	if len(packet) < 28 || packet[9] != 1 {
		return false
	}

	sourceIP := net.IPv4(packet[12], packet[13], packet[14], packet[15])
	destIP := net.IPv4(packet[16], packet[17], packet[18], packet[19])

	// Check for ICMP Echo Reply from 8.8.8.8 to 10.0.0.1
	if sourceIP.String() == "8.8.8.8" && destIP.String() == "10.0.0.1" {
		if len(packet) > 20 && packet[20] == 0 { // ICMP Echo Reply
			return true
		}
	}

	return false
}

func analyzeResponse(packet []byte) {
	if len(packet) < 28 {
		return
	}

	sourceIP := net.IPv4(packet[12], packet[13], packet[14], packet[15])
	destIP := net.IPv4(packet[16], packet[17], packet[18], packet[19])
	protocol := packet[9]
	icmpType := packet[20]

	fmt.Printf("🔍 Response Analysis:\n")
	fmt.Printf("   Source: %s\n", sourceIP)
	fmt.Printf("   Dest: %s\n", destIP)
	fmt.Printf("   Protocol: %d (ICMP)\n", protocol)
	fmt.Printf("   ICMP Type: %d (Echo Reply)\n", icmpType)
	fmt.Printf("   ✅ Valid ping response!\n")
}

func debugNetwork() {
	fmt.Println("\n🔍 Network Debug:")

	// Show interfaces
	fmt.Println("📡 Network interfaces:")
	cmd := exec.Command("ip", "addr", "show")
	output, _ := cmd.Output()
	fmt.Printf("%s\n", string(output))

	// Show routing
	fmt.Println("🛣️ Routing table:")
	cmd = exec.Command("ip", "route", "show")
	output, _ = cmd.Output()
	fmt.Printf("%s\n", string(output))

	// Test system connectivity
	fmt.Println("🌐 Testing system ping:")
	cmd = exec.Command("ping", "-c", "2", "-W", "3", "8.8.8.8")
	output, err := cmd.Output()
	if err != nil {
		fmt.Printf("❌ System ping failed: %v\n", err)
	} else {
		fmt.Printf("✅ System ping works:\n%s\n", string(output))
	}
}
func calculateChecksum(data []byte) uint16 {
	var sum uint32
	// Process 16-bit chunks
	for len(data) > 1 {
		sum += uint32(data[0])<<8 | uint32(data[1])
		data = data[2:]
	}
	// Add any remaining byte
	if len(data) > 0 {
		sum += uint32(data[0]) << 8
	}
	// Fold 32-bit sum to 16 bits
	for sum>>16 > 0 {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	return uint16(^sum)
}
