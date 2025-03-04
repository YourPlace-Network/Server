package network

// --- Pwnat --- //
//core.PwnnatServer()
//time.Sleep(3 * time.Second)
//payload := []byte{0x42, 0x42}
//core.PwnnatClient(net.ParseIP("73.20.167.212"), payload)

import (
	"fmt"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/robfig/cron/v3"
	"golang.org/x/net/icmp"
	"math/rand"
	"net"
	"time"
)

func PwnnatServer() {
	// samy.pl/pwnat/
	icmpServerEcho()
	crontab := cron.New(cron.WithSeconds())
	_, err := crontab.AddFunc("@every 30s", icmpServerEcho)
	if err != nil {
		panic("Could not start pwnnat server requests")
	}
	crontab.Start()
	serverListener()
}

func constructICMP(dstIP net.IP, ttl uint8, icmpType uint8, data []byte) []byte {
	eth := &layers.Ethernet{}
	eth.EthernetType = layers.EthernetTypeIPv4
	eth.SrcMAC = getMAC(getOutboundIP())
	eth.DstMAC = net.HardwareAddr{0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

	ip := &layers.IPv4{}
	ip.Version = 4
	ip.Protocol = layers.IPProtocolICMPv4
	ip.Flags = layers.IPv4DontFragment
	ip.SrcIP = getOutboundIP()
	ip.DstIP = dstIP
	ip.TTL = ttl

	icmp := &layers.ICMPv4{}
	icmp.TypeCode = layers.CreateICMPv4TypeCode(icmpType, 0) // 0x0800
	icmp.Id = uint16(rand.Uint32())
	icmp.Seq = 1
	icmp.Checksum = 0

	opts := gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}
	buf := gopacket.NewSerializeBuffer()
	err := gopacket.SerializeLayers(buf, opts, eth, ip, icmp, gopacket.Payload(data))
	if err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func sendPacket(packet []byte) {
	timeout := 30 * time.Second
	device, _, _, _, _ := getInterface()
	handle, err := pcap.OpenLive(device, 1024, false, timeout)
	if err != nil {
		panic(err)
	}
	defer handle.Close()
	err = handle.WritePacketData(packet)
	if err != nil {
		panic(err)
	}
}

func icmpServerEcho() {
	// https://github.com/margina757/probe/blob/32336a0fd7b1013825362767f4a22689ec183a93/probe/ping.go
	// https://github.com/wangzhezhe/gopacketlearn/blob/master/createpacket.go
	payload := []byte{0x70, 0x65, 0x65, 0x70, 0x65, 0x65, 0x70, 0x6f, 0x6f, 0x70, 0x6f, 0x6f}
	echoPacket := constructICMP(net.ParseIP("1.2.3.4"), 3, layers.ICMPv4TypeEchoRequest, payload)
	sendPacket(echoPacket)
}

func serverListener() {
	// https://stackoverflow.com/questions/2937123/implementing-icmp-ping-in-go
	fmt.Println("starting listener")
	/*if host.IsWindows() {
		fmt.Println("Can't perform pwnnat on Windows yet due to a Golang bug. See code comments for more.")
		return
	}*/
	listener, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	// ^^^ broken on Windows :(  https://github.com/golang/go/issues/38427
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	packetsChannel := make(chan *icmp.Echo)
	for {
		go func() {
			buff := make([]byte, 1500)
			size, _, err := listener.ReadFrom(buff)
			if err != nil {
				panic(err)
			}
			message, err := icmp.ParseMessage(58, buff[:size])
			if err != nil {
				panic(err)
			}
			body := message.Body.(*icmp.Echo)
			packetsChannel <- body
		}()
	}
}

func PwnnatClient(serverIP net.IP, tunneledPayload []byte) {
	childPacket := constructICMP(net.ParseIP("1.2.3.4"), 128, layers.ICMPv4TypeEchoRequest, tunneledPayload)
	parentPacket := constructICMP(serverIP, 128, layers.ICMPv4TypeTimeExceeded, childPacket)
	fmt.Println("send it..")
	sendPacket(parentPacket)
}
