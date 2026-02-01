package windowsclient

import (
	"fmt"
	"os/exec"
	"strings"
)

func getDefaultGateway() (string, error) {
	// PowerShell Command Explanation:
	// 1. Get-NetRoute: Fetch all routing table entries
	// 2. -DestinationPrefix "0.0.0.0/0": Filter for the "Default Gateway"
	// 3. Sort-Object RouteMetric: Put the "best" connection (lowest cost) first
	// 4. Select-Object -First 1: Pick only the best one
	// 5. -ExpandProperty NextHop: Print only the IP address

	psCommand := `Get-NetRoute -DestinationPrefix "0.0.0.0/0" | Sort-Object RouteMetric | Select-Object -First 1 -ExpandProperty NextHop`

	cmd := exec.Command("powershell", "-NoProfile", "-Command", psCommand)

	// Windows hides the console window for this command automatically
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get gateway: %v", err)
	}

	// Clean up the output (remove newlines/spaces)
	gatewayIP := strings.TrimSpace(string(output))

	if gatewayIP == "" {
		return "", fmt.Errorf("no default gateway found")
	}

	return gatewayIP, nil
}

func getDefaultInterface() (string, error) {
	// PowerShell Command Explanation:
	// 1. Get-NetRoute: Fetch all routing table entries
	// 2. -DestinationPrefix "0.0.0.0/0": Filter for the "Default Gateway"
	// 3. Sort-Object RouteMetric: Put the "best" connection (lowest cost) first
	// 4. Select-Object -First 1: Pick only the best one
	// 5. -ExpandProperty InterfaceAlias: Print the interface name

	psCommand := `Get-NetRoute -DestinationPrefix "0.0.0.0/0" | Sort-Object RouteMetric | Select-Object -First 1 -ExpandProperty InterfaceAlias`

	cmd := exec.Command("powershell", "-NoProfile", "-Command", psCommand)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get default interface: %v", err)
	}

	// Clean up the output (remove newlines/spaces)
	interfaceName := strings.TrimSpace(string(output))

	if interfaceName == "" {
		return "", fmt.Errorf("no default interface found")
	}

	return interfaceName, nil
}

func getDefaultInterfaceIP() (string, error) {
	// PowerShell Command Explanation:
	// 1. Get-NetIPAddress: Fetch all IP addresses
	// 2. -InterfaceAlias: Filter by the default interface name
	// 3. -AddressFamily IPv4: Only IPv4 addresses
	// 4. -ExpandProperty IPAddress: Print only the IP address

	// First get the default interface name
	interfaceName, err := getDefaultInterface()
	if err != nil {
		return "", err
	}

	psCommand := fmt.Sprintf(`Get-NetIPAddress -InterfaceAlias "%s" -AddressFamily IPv4 | Where-Object {$_.PrefixOrigin -ne "WellKnown"} | Select-Object -First 1 -ExpandProperty IPAddress`, interfaceName)

	cmd := exec.Command("powershell", "-NoProfile", "-Command", psCommand)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get interface IP: %v", err)
	}

	// Clean up the output
	interfaceIP := strings.TrimSpace(string(output))

	if interfaceIP == "" {
		return "", fmt.Errorf("no IP found for default interface")
	}

	return interfaceIP, nil
}

func getInterfaceIndex(interfaceName string) (string, error) {
	// Get the interface index (numeric ID) from the interface name
	psCommand := fmt.Sprintf(`(Get-NetAdapter -Name "%s").ifIndex`, interfaceName)

	cmd := exec.Command("powershell", "-NoProfile", "-Command", psCommand)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get interface index: %v", err)
	}

	interfaceIndex := strings.TrimSpace(string(output))

	if interfaceIndex == "" {
		return "", fmt.Errorf("no interface index found for %s", interfaceName)
	}

	return interfaceIndex, nil
}
