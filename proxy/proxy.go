package main

import (
	"comp7005_project/utils"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"os"
	"time"
)

type Key int

const ProxyKey Key = 0

type ProxyCtx struct {
	Socket                 *net.UDPConn
	ClientAddress          *net.UDPAddr
	ServerAddress          *net.UDPAddr
	ProxyAddress           *net.UDPAddr
	SIp, DIp, SPort, DPort string
	Data                   []byte

	ClientDropChance, ServerDropChance   int
	ClientDelayChance, ServerDelayChance int
	ClientDelayMin, ClientDelayMax       int
	ServerDelayMin, ServerDelayMax       int
}

func exit(proxyCtx *ProxyCtx) {
	fmt.Println("Exiting...")
	os.Exit(0)
}

func cleanup(proxyCtx *ProxyCtx) {
	if proxyCtx.Socket != nil {
		proxyCtx.Socket.Close()
	}

	exit(proxyCtx)
}

func receive(proxyCtx *ProxyCtx) {
	buffer := make([]byte, 1024)
	n, addr, err := proxyCtx.Socket.ReadFromUDP(buffer)
	if err != nil {
		fmt.Println(err)
	}
	proxyCtx.Data = buffer[0:n]

	proxyCtx.ClientAddress = addr

	packet, _ := utils.DecodePacket(proxyCtx.Data)

	if sendTo(fmt.Sprintf("%s:%d", addr.IP, addr.Port), fmt.Sprintf("%s:%d", proxyCtx.ServerAddress.IP, proxyCtx.ServerAddress.Port)) {
		fmt.Println("Recieved from server:", packet)
		dropChance := rand.Intn(100)

		if dropChance < proxyCtx.ServerDropChance {
			fmt.Println("Packet Dropped")
			receive(proxyCtx)
		}

		delayChance := rand.Intn(100)
		if delayChance < proxyCtx.ServerDelayChance {
			fmt.Println("Packet Delayed")
			time.Sleep(5 * time.Second)
		}

		sendToClient(proxyCtx)
	} else {
		fmt.Println("Recieved from client:", packet)
		dropChance := rand.Intn(100)

		if dropChance < proxyCtx.ClientDropChance {
			fmt.Println("Packet Dropped")
			receive(proxyCtx)
		}

		delayChance := rand.Intn(100)
		if delayChance < proxyCtx.ClientDelayChance {
			fmt.Println("Packet Delayed")
			time.Sleep(5 * time.Second)
		}

		sendToServer(proxyCtx)
	}
}

func dropPacket(proxyCtx *ProxyCtx) {
}

func delayPacket(proxyCtx *ProxyCtx) {
}

func sendTo(ip string, server string) bool {
	if ip == server {
		return true
	} else {
		return false
	}
}

func sendToClient(proxyCtx *ProxyCtx) {
	packet, err := utils.DecodePacket(proxyCtx.Data)
	if err != nil {
		fmt.Println(err)
		cleanup(proxyCtx)
	}

	addr, err := net.ResolveUDPAddr("udp", packet.DstAddr)
	if err != nil {
		fmt.Println(err)
		cleanup(proxyCtx)
	}

	_, err = proxyCtx.Socket.WriteToUDP([]byte(proxyCtx.Data), addr)
	if err != nil {
		fmt.Println(err)
		cleanup(proxyCtx)
	}
	receive(proxyCtx)
}

func sendToServer(proxyCtx *ProxyCtx) {
	_, err := proxyCtx.Socket.WriteToUDP([]byte(proxyCtx.Data), proxyCtx.ServerAddress)
	if err != nil {
		fmt.Println(err)
		cleanup(proxyCtx)
	}
	receive(proxyCtx)
}

func connectToServer(proxyCtx *ProxyCtx) {
	s, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%s", proxyCtx.DIp, proxyCtx.DPort))
	if err != nil {
		fmt.Println(err)
		exit(proxyCtx)
	}

	proxyCtx.ServerAddress = s
	_, err = net.DialUDP("udp4", nil, s)
	if err != nil {
		cleanup(proxyCtx)
	}

	fmt.Println("Connected to UDP server at", proxyCtx.ServerAddress)

	receive(proxyCtx)
}

// both
func bind_socket(proxyCtx *ProxyCtx) {
	s, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%s", proxyCtx.SIp, proxyCtx.SPort))
	if err != nil {
		fmt.Println(err)
		exit(proxyCtx)
	}

	connection, err := net.ListenUDP("udp4", s)
	if err != nil {
		fmt.Println(err)
		exit(proxyCtx)
	}

	proxyCtx.ProxyAddress = s
	proxyCtx.Socket = connection

	fmt.Println("The UDP server is", proxyCtx.ProxyAddress)

	connectToServer(proxyCtx)
}

func parseArgs(proxyCtx *ProxyCtx) {
	clientDropRate := flag.Int("cdrop", 0, "client drop")
	serverDropRate := flag.Int("sdrop", 0, "server drop")

	clientDelayChance := flag.Int("cdelay", 0, "client delay chance")
	serverDelayChance := flag.Int("sdelay", 0, "server delay chance")

	clientDelayMin := flag.Int("cmin", 0, "client delay min")
	clientDelayMax := flag.Int("cmax", 0, "client delay max")
	serverDelayMin := flag.Int("smin", 0, "server delay min")
	serverDelayMax := flag.Int("smax", 0, "server delay max")

	flag.Parse()

	if len(flag.Args()) < 4 {
		exit(proxyCtx)
	}

	proxyCtx.SIp = flag.Args()[0]
	proxyCtx.SPort = flag.Args()[1]
	proxyCtx.DIp = flag.Args()[2]
	proxyCtx.DPort = flag.Args()[3]

	proxyCtx.ClientDropChance = *clientDropRate
	proxyCtx.ServerDropChance = *serverDropRate

	proxyCtx.ClientDelayChance = *clientDelayChance
	proxyCtx.ServerDelayChance = *serverDelayChance

	proxyCtx.ClientDelayMin = *clientDelayMin
	proxyCtx.ClientDelayMax = *clientDelayMax
	proxyCtx.ServerDelayMin = *serverDelayMin
	proxyCtx.ServerDelayMax = *serverDelayMax

	bind_socket(proxyCtx)
}

func main() {
	proxyCtx := ProxyCtx{}
	parseArgs(&proxyCtx)
}
