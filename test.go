package main

import (
	"YourPlace/src/core/network"
	"fmt"
	"os/exec"
)

func main() {
	network.DownloadTor()

	// Try to run Tor binary directly with just --help to see if it works
	torPath := "/Users/nops/Library/Caches/YourPlace/tor/tor/tor"
	cmd := exec.Command(torPath, "--help")
	output, err := cmd.Output()
	if err != nil {
		fmt.Printf("Direct Tor execution failed: %v\n", err)
	} else {
		fmt.Printf("Tor help output length: %d bytes\n", len(output))
	}

	/*address, err := network.StartTorHiddenService()
	if err != nil {
		panic(err)
	}
	println("Tor hidden service started at:", address)*/
}
