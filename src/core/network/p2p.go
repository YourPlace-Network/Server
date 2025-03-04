package network

// --- P2P Server --- //

import (
	"fmt"
	"net"
)

func P2PServer(port int) {
	listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		fmt.Printf("Could not start P2P server on port %d\n", port)
		return
	}
	defer listener.Close()
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("Socket error: %s\n", err)
		}
		go handleP2PServerConnection(conn)
	}
}

func handleP2PServerConnection(conn net.Conn) {
	fmt.Println("Handinlg p2p connection")
}
