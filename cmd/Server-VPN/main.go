//go:build linux
// +build linux

package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"syscall"
	"unsafe"
)

var TunFd int
var UDPConn *net.UDPConn
var ClientAddr *net.UDPAddr

const (
	TUNSETIFF = 0x400454ca
	IFF_TUN   = 0x0001
	IFF_NO_PI = 0x1000
)

type ifreq struct {
	name  [16]byte
	flags uint16
	_     [22]byte
}

func main() {
	var err error

	// Create TUN interface
	TunFd, err = CreateTunInterface("tun0")
	if err != nil {
		panic(fmt.Sprintf("Failed to create TUN interface: %v", err))
	}
	defer syscall.Close(TunFd)

	// Configure TUN interface
	err = ConfigureTunInterface()
	if err != nil {
		panic(fmt.Sprintf("Failed to configure TUN interface: %v", err))
	}

	// Setup NAT and forwarding
	err = SetupNATAndForwarding()
	if err != nil {
		panic(fmt.Sprintf("Failed to setup NAT: %v", err))
	}

	// Create UDP listener
	UDPConn, err = net.ListenUDP("udp", &net.UDPAddr{
		IP:   net.ParseIP("0.0.0.0"),
		Port: 8080,
	})
	if err != nil {
		panic(fmt.Sprintf("Could not create UDP listener: %v", err))
	}
	defer UDPConn.Close()

	fmt.Println("✅ Server ready - UDP listening on port 8080")
	fmt.Println("✅ TUN interface tun0 created and configured")

	// Run goroutines
	go func() {
		err := ListenForPackets(UDPConn)
		if err != nil {
			fmt.Printf("❌ ListenForPackets error: %v\n", err)
		}
	}()

	go func() {
		err := ReadFromTunAndSendToClient()
		if err != nil {
			fmt.Printf("❌ ReadFromTunAndSendToClient error: %v\n", err)
		}
	}()

	fmt.Println("🚀 VPN Server running... Press Ctrl+C to stop")
	select {} // Block forever
}

func CreateTunInterface(name string) (int, error) {
	fmt.Printf("Creating TUN interface %s\n", name)

	fd, err := syscall.Open("/dev/net/tun", os.O_RDWR, 0)
	if err != nil {
		return -1, fmt.Errorf("failed to open /dev/net/tun: %v", err)
	}

	var ifr ifreq
	copy(ifr.name[:], name)
	ifr.flags = IFF_TUN | IFF_NO_PI

	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), uintptr(TUNSETIFF), uintptr(unsafe.Pointer(&ifr)))
	if errno != 0 {
		syscall.Close(fd)
		return -1, fmt.Errorf("ioctl TUNSETIFF failed: %v", errno)
	}

	fmt.Printf("✅ TUN interface created (fd: %d)\n", fd)
	return fd, nil
}

func ConfigureTunInterface() error {
	fmt.Println("Configuring TUN interface...")

	// Bring interface up with IP
	cmd := exec.Command("ip", "addr", "add", "10.8.0.1/24", "dev", "tun0")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set IP: %v", err)
	}

	cmd = exec.Command("ip", "link", "set", "dev", "tun0", "up")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to bring interface up: %v", err)
	}

	fmt.Println("✅ TUN interface configured with IP 10.8.0.1/24")
	return nil
}

func SetupNATAndForwarding() error {
	fmt.Println("Setting up NAT and packet forwarding...")

	// Enable IP forwarding
	cmd := exec.Command("sysctl", "-w", "net.ipv4.ip_forward=1")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to enable IP forwarding: %v", err)
	}

	// Setup NAT masquerading (assumes eth0 as main interface, change if needed)
	cmd = exec.Command("iptables", "-t", "nat", "-A", "POSTROUTING", "-s", "10.8.0.0/24", "-o", "eth0", "-j", "MASQUERADE")
	if err := cmd.Run(); err != nil {
		fmt.Printf("⚠️ Warning: iptables NAT rule may already exist or eth0 not found: %v\n", err)
	}

	// Allow forwarding
	cmd = exec.Command("iptables", "-A", "FORWARD", "-i", "tun0", "-j", "ACCEPT")
	if err := cmd.Run(); err != nil {
		fmt.Printf("⚠️ Warning: iptables forward rule may already exist: %v\n", err)
	}

	cmd = exec.Command("iptables", "-A", "FORWARD", "-o", "tun0", "-j", "ACCEPT")
	if err := cmd.Run(); err != nil {
		fmt.Printf("⚠️ Warning: iptables forward rule may already exist: %v\n", err)
	}

	fmt.Println("✅ NAT and forwarding configured")
	return nil
}
func ListenForPackets(conn *net.UDPConn) error {
	fmt.Printf("Listening for UDP packets from client\n")

	buffer := make([]byte, 65535)

	for {
		n, addr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			fmt.Printf("❌ Error reading UDP packet: %v\n", err)
			continue
		}

		ClientAddr = addr

		if n > 0 {
			packet := buffer[:n]

			// Extract destination IP for logging
			if len(packet) >= 20 {
				destIP := net.IPv4(packet[16], packet[17], packet[18], packet[19])
				fmt.Printf("📥 Received packet from client, dest: %s\n", destIP)
			}

			// Write packet to TUN interface - kernel will route it
			err = WriteToTun(packet)
			if err != nil {
				fmt.Printf("❌ Error writing to TUN: %v\n", err)
			}
		}
	}
}

func WriteToTun(packet []byte) error {
	n, err := syscall.Write(TunFd, packet)
	if err != nil {
		return fmt.Errorf("failed to write to TUN: %v", err)
	}
	fmt.Printf("✅ Wrote %d bytes to TUN interface\n", n)
	return nil
}

func ReadFromTunAndSendToClient() error {
	fmt.Println("Reading packets from TUN interface...")
	buffer := make([]byte, 65535)

	for {
		n, err := syscall.Read(TunFd, buffer)
		if err != nil {
			fmt.Printf("❌ Error reading from TUN: %v\n", err)
			continue
		}

		if n > 0 {
			packet := buffer[:n]

			// Extract source and dest IPs for logging
			if len(packet) >= 20 {
				sourceIP := net.IPv4(packet[12], packet[13], packet[14], packet[15])
				destIP := net.IPv4(packet[16], packet[17], packet[18], packet[19])
				fmt.Printf("📤 Read from TUN: %s -> %s\n", sourceIP, destIP)
			}

			// Send packet back to client via UDP
			err = SendPacketToClient(packet)
			if err != nil {
				fmt.Printf("❌ Error sending to client: %v\n", err)
			}
		}
	}
}

func SendPacketToClient(packet []byte) error {
	if ClientAddr == nil {
		return fmt.Errorf("no client address available")
	}

	_, err := UDPConn.WriteToUDP(packet, ClientAddr)
	if err != nil {
		return fmt.Errorf("failed to send UDP packet: %v", err)
	}

	fmt.Printf("✅ Sent packet to client (%d bytes)\n", len(packet))
	return nil
}
