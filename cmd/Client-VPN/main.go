package main

import (
	"fmt"
	"net"

	"github.com/varun0310t/VPN/internal/tunnel"
)

func main() {
	fmt.Print("Starting the vpn server")
	ifce := tunnel.CreateTUN()

	defer func() {
		fmt.Printf("Cleaning up the mess...")
		tunnel.RestoreOriginalRoute()
		ifce.Close()
		fmt.Print("Routes restored - internet should work now")
	}()

	tunnel.SetDefaultRoute()

	buffer := make([][]byte, 1)
	buffer[0] = make([]byte, 1500)
	count := 0
	lengths := make([]int, 1)

	fmt.Println("\nAnalyzing captured packets...")

	for {
		n, err := ifce.Read(buffer, lengths, 0)
		if err != nil {
			fmt.Printf("Read error: %v\n", err)
			continue
		}

		if n > 0 {
			count++
			packet := buffer[0][:lengths[0]]

			// Analyze the packet
			if len(packet) >= 20 { // Minimum IP header
				version := packet[0] >> 4
				if version == 4 {
					// Extract destination IP
					destIP := net.IPv4(packet[16], packet[17], packet[18], packet[19])
					protocol := packet[9]

					protocolName := "Unknown"
					switch protocol {
					case 1:
						protocolName = "ICMP"
					case 6:
						protocolName = "TCP"
					case 17:
						protocolName = "UDP"
					}

					fmt.Printf("Packet %d: %s to %s (%d bytes)\n",
						count, protocolName, destIP, lengths[0])
				}
			}

		}
	}
}
