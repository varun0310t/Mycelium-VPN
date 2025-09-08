package tunnel

import (
	"log"

	"golang.zx2c4.com/wireguard/tun"
)

func CreateTUN() tun.Device {
	device, err := tun.CreateTUN("VPN-Interface", 1420)

	if err != nil {
		log.Printf("Failed to create TUN interface: %v", err)
		log.Println("Hint: Ensure wintun.dll is in the same directory as the executable.")
		return nil
	}

	name, err := device.Name()
	if err != nil {
		log.Printf("Failed to get device name: %v", err)
		device.Close()
		return nil
	}

	log.Printf("TUN Interface created: %s", name)
	return device
}
