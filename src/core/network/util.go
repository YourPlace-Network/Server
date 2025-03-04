package network

import (
	"YourPlace/src/core"
	"YourPlace/src/core/services"
	"github.com/google/gopacket/pcap"
	"github.com/jackpal/gateway"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

/* ------------- Util ------------- */

func getOutboundIP() net.IP {
	conn, err := net.Dial("udp", "1.1.1.1:80")
	if err != nil {
		panic(err)
	}
	defer func(conn net.Conn) {
		err = conn.Close()
		if err != nil {
			panic(err)
		}
	}(conn)
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP
}
func getInterface() (device string, mac *net.HardwareAddr, gwip *net.IP, src *net.IP, err error) {
	newGwip, err := gateway.DiscoverGateway()
	if err != nil {
		panic(err)
	}
	device, myIP := selectDevice()
	newMac := getMAC(myIP)
	return device, &newMac, &newGwip, &myIP, nil
}
func getMAC(ip net.IP) net.HardwareAddr {
	interfaces, err := net.Interfaces()
	if err != nil {
		panic(err)
	}
	for _, interf := range interfaces {
		if addrs, err := interf.Addrs(); err == nil {
			for _, addr := range addrs {
				if strings.Split(addr.String(), "/")[0] == ip.String() {
					return interf.HardwareAddr
				}
			}
		}
	}
	return net.HardwareAddr{0, 0, 0, 0, 0, 0}
}
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
		core.LogDebug("Could not connect to TCP port: " + strconv.Itoa(port) + " Host: " + host)
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
func GetHTTPRoundTripTime(url string) time.Duration {
	start := time.Now()
	resp, err := http.Get(url)
	if err != nil {
		core.LogWarn("Could not get HTTP round trip time: " + err.Error())
		return time.Duration(0)
	}
	defer resp.Body.Close()
	duration := time.Since(start)
	return duration
}
func GetTCPRoundTripTime(host string, port int) time.Duration {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), 60*time.Second)
	if err != nil {
		core.LogWarn("Could not get TCP round trip time: " + err.Error())
		return time.Duration(0)
	}
	defer func(conn net.Conn) {
		err = conn.Close()
		if err != nil {
			core.LogError("Could not close TCP connection")
		}
	}(conn)
	duration := time.Since(start)
	return duration
}
func selectDevice() (_device string, ip net.IP) {
	// https://gist.github.com/FlameInTheDark/b1957b95a89493ec6ce346bad156dc61#file-main-go
	localIP := getOutboundIP()
	devices, err := pcap.FindAllDevs()
	if err != nil {
		panic(err)
	}
	var name string
	for _, device := range devices {
		for _, address := range device.Addresses {
			if localIP != nil {
				if address.IP.String() == localIP.String() {
					name = device.Name
				}
			} else if address.IP.String() != "127.0.0.1" && !strings.Contains(device.Description, "Loopback") {
				name = device.Name
			}
		}
	}
	return name, localIP
}
