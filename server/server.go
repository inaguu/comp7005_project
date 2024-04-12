package main

import (
	"comp7005_project/utils"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strconv"
	"time"
)

type Key int

type ServerCtx struct {
	Socket        *net.UDPConn
	ClientAddress *net.UDPAddr
	Ip, Port      string
	Packet        utils.Packet
}

const (
	INPUT_ERROR     = "Usage: <filename> <ip address> <port_number>"
	ServerKey   Key = 0
)

func random(min, max int) int {
	return rand.Intn(max-min) + min
}

func exit(serverCtx *ServerCtx) {
	fmt.Println("Exiting...")
	os.Exit(0)
}

func cleanup(serverCtx *ServerCtx) {
	if serverCtx.Socket != nil {
		serverCtx.Socket.Close()
	}
	exit(serverCtx)
}

func send(serverCtx *ServerCtx) {
	rand.Seed(time.Now().Unix())

	data := []byte(strconv.Itoa(random(1, 1001)))
	fmt.Printf("Sent: %s\n", string(data))

	_, err := serverCtx.Socket.WriteToUDP(data, serverCtx.ClientAddress)
	if err != nil {
		fmt.Println(err)
		cleanup(serverCtx)
	}

	receive(serverCtx)
}

func receive(serverCtx *ServerCtx) {
	buffer := make([]byte, 1024)

	n, addr, err := serverCtx.Socket.ReadFromUDP(buffer)
	if err != nil {
		fmt.Println(err)
		cleanup(serverCtx)
	}

	bytes := buffer[0:n]

	packet, err := utils.DecodePacket(bytes)
	if err != nil {
		fmt.Println(err)
		cleanup(serverCtx)
	}

	serverCtx.ClientAddress = addr

	if packet.Header.Flags.SYN {
		fmt.Println("Received -> SYN")
		serverCtx.Packet = packet
		sendSynAck(serverCtx)
	} else {
		fmt.Println("Received ->", packet.Data)
		send(serverCtx)
	}
}

func sendSynAck(serverCtx *ServerCtx) {
	packet := utils.Packet{
		SrcAddr: serverCtx.Packet.SrcAddr,
		DstAddr: serverCtx.Packet.DstAddr,
		Header:  utils.Header{Flags: utils.Flags{SYN: true, ACK: true}, Seq: 0, Ack: 0, Len: 0},
	}

	bytes, err := utils.EncodePacket(packet)
	if err != nil {
		fmt.Println(err)
		return
	}

	_, err = serverCtx.Socket.WriteToUDP(bytes, serverCtx.ClientAddress)
	if err != nil {
		fmt.Println(err)
		cleanup(serverCtx)
	}

	fmt.Println("Sent -> SYN/ACK")
	waitForAck(serverCtx)
}

func waitForAck(serverCtx *ServerCtx) {
	buffer := make([]byte, 1024)

	n, _, err := serverCtx.Socket.ReadFromUDP(buffer)
	if err != nil {
		fmt.Println(err)
		cleanup(serverCtx)
	}

	bytes := buffer[0:n]

	packet, err := utils.DecodePacket(bytes)
	if err != nil {
		fmt.Println(err)
		cleanup(serverCtx)
	}

	if packet.Header.Flags.ACK {
		fmt.Println("Received -> ACK")
		receive(serverCtx)
	} else {
		fmt.Println("The packet wasn't an ACK packet")
	}
}

func bindSocket(serverCtx *ServerCtx) {
	s, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%s", serverCtx.Ip, serverCtx.Port))
	if err != nil {
		fmt.Println(err)
		exit(serverCtx)
	}

	connection, err := net.ListenUDP("udp4", s)
	if err != nil {
		fmt.Println(err)
		exit(serverCtx)
	}

	serverCtx.Socket = connection

	receive(serverCtx)
}

func parseArgs(serverCtx *ServerCtx) {
	if len(os.Args) < 3 {
		fmt.Println(INPUT_ERROR)
		exit(serverCtx)
	}

	serverCtx.Ip = os.Args[1]
	serverCtx.Port = os.Args[2]

	bindSocket(serverCtx)
}

func main() {
	serverCtx := ServerCtx{}
	parseArgs(&serverCtx)
}
