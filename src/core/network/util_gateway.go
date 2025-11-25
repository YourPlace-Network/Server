//go:build gateway

package network

import (
	"YourPlace/src/core"
	"YourPlace/src/core/services"
	"net"
	"strconv"
	"time"
)

/* ------------- Util (Gateway Mode) ------------- */

func GetPublicIP() (net.IP, error) {
	// http://ifconfig.me
	// http://api.ipify.org
	// http://ipecho.net/plain
	// http://v4.ident.me
	// https://myipv4.p1.opendns.com/get_my_ip (openDNS)
	// https://myip.dnsomatic.com/ (openDNS)
	// IP=$(curl -s "$provider")
	//    if [[ $IP =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
	openDNSPublicIP, err := services.OpenDNSGetPublicIP()
	if err != nil {
		return nil, core.LogErrorReturn("Could not get public IP from openDNS: " + err.Error())
	}
	return openDNSPublicIP, nil
}
func IsTCPPortOpen(host string, port int) bool {
	const timeout = 10 * time.Second
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), timeout)
	if err != nil {
		return false
	}
	defer func(conn net.Conn) {
		err = conn.Close()
		if err != nil {
			core.LogError("Could not close TCP connection")
		}
	}(conn)
	return true
}
func IsInternetConnected() bool {
	open := IsTCPPortOpen("google.com", 443)
	open2 := IsTCPPortOpen("cloudflare.com", 443)
	open3 := IsTCPPortOpen("microsoft.com", 443)
	return open || open2 || open3
}
