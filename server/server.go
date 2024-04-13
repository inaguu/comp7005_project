package main

import (
	"comp7005_project/utils"
	"fmt"
	"math/rand"
	"net"
	"os"
)

type ServerCtx struct {
	Socket        *net.UDPConn
	ClientAddress *net.UDPAddr

	packetsSent, packetsReceived []utils.Packet

	Ip, Port string
	Packet   utils.Packet
}

const (
	INPUT_ERROR = "Usage: <filename> <ip address> <port_number>"
)

func packetString(packet utils.Packet) string {
	return fmt.Sprintf("[Seq: %d | Ack: %d]", packet.Header.Seq, packet.Header.Ack)
}

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

func sendFinAck(serverCtx *ServerCtx) {
	packet := utils.Packet{
		SrcAddr: serverCtx.Packet.SrcAddr,
		DstAddr: serverCtx.Packet.DstAddr,
		Header:  utils.Header{Flags: utils.Flags{FIN: true, ACK: true}, Seq: serverCtx.Packet.Header.Ack, Ack: serverCtx.Packet.Header.Seq + serverCtx.Packet.Header.Len, Len: 1},
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

	serverCtx.packetsSent = append(serverCtx.packetsSent, packet)
	fmt.Println("Sent -> FIN/ACK with packet:", packetString(packet))

	waitForAck(serverCtx)
}

func send(serverCtx *ServerCtx) {
	lastPacketReceived := serverCtx.packetsReceived[len(serverCtx.packetsReceived)-1]
	lastPacketSent := serverCtx.packetsSent[len(serverCtx.packetsSent)-1]

	packet := utils.Packet{
		SrcAddr: serverCtx.Packet.SrcAddr,
		DstAddr: serverCtx.Packet.DstAddr,
		Header:  utils.Header{Flags: utils.Flags{SYN: true, ACK: true}, Seq: lastPacketReceived.Header.Ack, Ack: lastPacketSent.Header.Ack + lastPacketReceived.Header.Len, Len: 1},
	}

	if lastPacketReceived.Header.Flags.SYN {
		packet.Header.Ack = 1
		packet.Header.Seq = 0
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

	serverCtx.packetsSent = append(serverCtx.packetsSent, packet)
	fmt.Println("\nSend -> ACK with packet:", packetString(packet))

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

	serverCtx.packetsReceived = append(serverCtx.packetsReceived, packet)
	serverCtx.ClientAddress = addr
	serverCtx.Packet = packet

	if packet.Header.Flags.SYN {
		fmt.Println("Received -> SYN with packet:", packetString(packet))
		sendSynAck(serverCtx)
	} else if packet.Header.Flags.FIN {
		fmt.Println("Received -> FIN with packet:", packetString(packet))
		sendFinAck(serverCtx)
	} else {

		// if packet.Header.

		fmt.Printf("Received -> %s with packet: %s", "packet.Data", packetString(packet))
		send(serverCtx)
	}
}

func sendSynAck(serverCtx *ServerCtx) {
	packet := utils.Packet{
		SrcAddr: serverCtx.Packet.SrcAddr,
		DstAddr: serverCtx.Packet.DstAddr,
		Header:  utils.Header{Flags: utils.Flags{SYN: true, ACK: true}, Seq: 0, Ack: 1, Len: 0},
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

	serverCtx.packetsSent = append(serverCtx.packetsSent, packet)
	fmt.Println("Sent -> SYN/ACK with packet:", packetString(packet))
	waitForAck(serverCtx)
}

func waitForAck(serverCtx *ServerCtx) {
	buffer := make([]byte, 1024)

	n, _, err := serverCtx.Socket.ReadFromUDP(buffer)
	if err != nil {
		if netError, ok := err.(net.Error); ok && netError.Timeout() {
			fmt.Println("Timeout waiting for ACK -> restarting connection")
			receive(serverCtx)
		} else {
			fmt.Println(err)
			cleanup(serverCtx)
		}
	}

	bytes := buffer[0:n]

	packet, err := utils.DecodePacket(bytes)
	if err != nil {
		fmt.Println(err)
		cleanup(serverCtx)
	}

	serverCtx.packetsReceived = append(serverCtx.packetsReceived, packet)

	if packet.Header.Flags.ACK {
		fmt.Println("Received -> ACK with packet:", packetString(packet))
		receive(serverCtx)
	} else {
		fmt.Println("The packet wasn't an ACK packet -> restarting connection")
		receive(serverCtx)
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
}

func parseArgs(serverCtx *ServerCtx) {
	if len(os.Args) < 3 {
		fmt.Println(INPUT_ERROR)
		exit(serverCtx)
	}

	serverCtx.Ip = os.Args[1]
	serverCtx.Port = os.Args[2]

	fmt.Printf("The UDP server is %s:%s\n", serverCtx.Ip, serverCtx.Port)
}

func main() {
	serverCtx := ServerCtx{}
	parseArgs(&serverCtx)
	bindSocket(&serverCtx)
	receive(&serverCtx)
}
