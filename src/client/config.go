package client

import (
	"encoding/json"
	"fmt"
	"os"
)

type ClientConfig struct {
	Password string `json:"password"`
}

func loadClientConfig() (*ClientConfig, error) {
	path := "./src/config/ClientConfig.json"

	data, err := os.ReadFile(path)
	if err != nil {
		// Return default config if file doesn't exist

		fmt.Printf(" Config file not found at %s, using default config\n", path)
		if os.IsNotExist(err) {
			return &ClientConfig{
				Password: "VPN1234",
			}, nil
		}
		return nil, err
	}

	var config ClientConfig
	err = json.Unmarshal(data, &config)
	return &config, err
}
