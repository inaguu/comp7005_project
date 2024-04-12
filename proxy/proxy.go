package main

import (
	"fmt"
	"net"
	"os"
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

var check = true

func receive(proxyCtx *ProxyCtx) {
	buffer := make([]byte, 1024)
	n, addr, err := proxyCtx.Socket.ReadFromUDP(buffer)
	if err != nil {
		fmt.Println(err)
	}
	proxyCtx.Data = buffer[0:n]

	if check {
		proxyCtx.ClientAddress = addr
		check = false
	}

	fmt.Println("Data:", proxyCtx.Data)

	if sendTo(fmt.Sprintf("%s:%d", addr.IP, addr.Port), fmt.Sprintf("%s:%d", proxyCtx.ServerAddress.IP, proxyCtx.ServerAddress.Port)) {
		sendToClient(proxyCtx)
	} else {
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
	_, err := proxyCtx.Socket.WriteToUDP([]byte(proxyCtx.Data), proxyCtx.ClientAddress)
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
	if len(os.Args) < 5 {
		exit(proxyCtx)
	}

	proxyCtx.SIp = os.Args[1]
	proxyCtx.SPort = os.Args[2]
	proxyCtx.DIp = os.Args[3]
	proxyCtx.DPort = os.Args[4]

	bind_socket(proxyCtx)
}

func main() {
	proxyCtx := ProxyCtx{}
	parseArgs(&proxyCtx)
}
