package main

import (
	"github.com/varun0310t/VPN/internal/tunnel"
)

func main() {
	ifce := tunnel.CreateTUN()
	ifce.Close()
}
