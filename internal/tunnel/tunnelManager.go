package tunnel

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"golang.zx2c4.com/wireguard/tun"
)

var originalGateway string
var originalInterface string

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

	// Configure the TUN interface with an IP address
	if err := configureTUNInterface(name); err != nil {
		log.Printf("Failed to configure TUN interface: %v", err)
		device.Close()
		return nil
	}
	// Give the interface a moment to be ready
	time.Sleep(5 * time.Second)
	return device
}

func SetDefaultRoute() error {

	if err := storeOriginalRoute(); err != nil {
		return err
	}

	//route to the vpn server
	cmd := exec.Command("route", "add", "0.0.0.1", "mask", "255.255.255.255", originalGateway)
	cmd.Run()

	//delete default route
	cmd = exec.Command("route", "delete", "0.0.0.0")
	if err := cmd.Run(); err != nil {
		return err
	}

	// Add new default route through TUN
	cmd = exec.Command("route", "add", "0.0.0.0", "mask", "0.0.0.0", "10.0.0.1", "metric", "1")
	if err := cmd.Run(); err != nil {
		// If this fails, restore original route
		fmt.Print("routing failed going back to drfault")
		RestoreOriginalRoute()
		return err
	}

	log.Println("Default route set to TUN interface")

	return nil
}
func storeOriginalRoute() error {
	cmd := exec.Command("route", "print", "0.0.0.0")
	output, err := cmd.Output()
	if err != nil {
		return err
	}
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "0.0.0.0") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				originalGateway = parts[2]
				log.Printf("Stored original gateway: %s", originalGateway)
				break
			}
		}
	}
	return nil
}

func RestoreOriginalRoute() error {
	if originalGateway == "" {
		log.Println("No original gateway stored - cannot restore")
		return fmt.Errorf("no original gateway stored")
	}

	//delete VPN route
	cmd := exec.Command("route", "delete", "0.0.0.0")
	cmd.Run()

	//Restore original route
	cmd = exec.Command("route", "add", "0.0.0.0", "mask", "0.0.0.0", originalGateway)

	if err := cmd.Run(); err != nil {
		log.Printf("CRITICAL: Failed to restore original route. Manually run: route add 0.0.0.0 mask 0.0.0.0 %s", originalGateway)
		return err
	}

	return nil
}

func configureTUNInterface(interfaceName string) error {
	cmd := exec.Command("netsh", "interface", "ip", "set", "address", "name="+interfaceName, "static", "10.0.0.2", "255.255.255.0")
	if err := cmd.Run(); err != nil {
		log.Printf("Failed to set IP address: %v", err)
		return err
	}

	// Set DNS servers
	cmd = exec.Command("netsh", "interface", "ip", "set", "dns",
		"name="+interfaceName, "static", "8.8.8.8")
	if err := cmd.Run(); err != nil {
		log.Printf("Failed to set DNS: %v", err)
		return err
	}

	// Add secondary DNS
	cmd = exec.Command("netsh", "interface", "ip", "add", "dns",
		"name="+interfaceName, "8.8.4.4", "index=2")
	cmd.Run()
	log.Printf("TUN interface %s configured with IP 10.0.0.2", interfaceName)
	return nil
}
