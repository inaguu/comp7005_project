package main

import (
	"comp7005_project/utils"
	"fmt"
	"net"
	"os"
)

type Key int

const ClientKey Key = 0

type ClientCtx struct {
	Socket   *net.UDPConn
	Address  string
	FilePath string
	Data     string
	Packet   utils.Packet
}

func exit(clientCtx *ClientCtx) {
	fmt.Println("Exiting...")
	os.Exit(0)
}

func cleanup(clientCtx *ClientCtx) {
	if clientCtx.Socket != nil {
		clientCtx.Socket.Close()
	}

	exit(clientCtx)
}

func sendFinalAck(clientCtx *ClientCtx) {
	packet := utils.Packet{
		SrcAddr: clientCtx.Address,
		DstAddr: clientCtx.Socket.LocalAddr().String(),
		Header:  utils.Header{Flags: utils.Flags{ACK: true}, Seq: clientCtx.Packet.Header.Ack, Ack: clientCtx.Packet.Header.Seq + clientCtx.Packet.Header.Len, Len: 1},
	}

	bytes, err := utils.EncodePacket(packet)
	if err != nil {
		fmt.Println(err)
		return
	}

	_, err = clientCtx.Socket.Write(bytes)
	if err != nil {
		fmt.Println(err)
		cleanup(clientCtx)
	}
	fmt.Println("Sent -> ACK with packet: ", packet)

	cleanup(clientCtx)
}

func waitForFinAck(clientCtx *ClientCtx) {
	buffer := make([]byte, 1024)

	n, _, err := clientCtx.Socket.ReadFromUDP(buffer)
	if err != nil {
		fmt.Println(err)
		cleanup(clientCtx)
	}

	bytes := buffer[0:n]

	packet, err := utils.DecodePacket(bytes)
	if err != nil {
		fmt.Println(err)
		cleanup(clientCtx)
	}

	clientCtx.Packet = packet

	if packet.Header.Flags.FIN && packet.Header.Flags.ACK {
		fmt.Println("Received -> FIN/ACK with packet: ", packet)
		sendFinalAck(clientCtx)
	} else {
		fmt.Println("The packet wasn't a FIN/ACK")
	}
}

func sendFin(clientCtx *ClientCtx) {
	packet := utils.Packet{
		SrcAddr: clientCtx.Address,
		DstAddr: clientCtx.Socket.LocalAddr().String(),
		Header:  utils.Header{Flags: utils.Flags{FIN: true}, Seq: clientCtx.Packet.Header.Ack, Ack: clientCtx.Packet.Header.Seq + clientCtx.Packet.Header.Len, Len: 1},
	}

	bytes, err := utils.EncodePacket(packet)
	if err != nil {
		fmt.Println(err)
		return
	}

	_, err = clientCtx.Socket.Write(bytes)
	if err != nil {
		fmt.Println(err)
		cleanup(clientCtx)
	}
	fmt.Println("Sent -> FIN with packet: ", packet)
	waitForFinAck(clientCtx)
}

func receive(clientCtx *ClientCtx) {
	buffer := make([]byte, 1024)

	n, _, err := clientCtx.Socket.ReadFromUDP(buffer)
	if err != nil {
		fmt.Println(err)
		cleanup(clientCtx)
	}

	bytes := buffer[0:n]

	packet, err := utils.DecodePacket(bytes)
	if err != nil {
		fmt.Println(err)
		cleanup(clientCtx)
	}

	clientCtx.Packet = packet

	fmt.Println("\nReceived -> ACK with packet: ", packet)

	sendFin(clientCtx)
}

func send(clientCtx *ClientCtx) {
	packet := utils.Packet{
		SrcAddr: clientCtx.Address,
		DstAddr: clientCtx.Socket.LocalAddr().String(),
		Header:  utils.Header{Flags: utils.Flags{PSH: true, ACK: true}, Seq: clientCtx.Packet.Header.Ack, Ack: clientCtx.Packet.Header.Seq + clientCtx.Packet.Header.Len + 1, Len: uint32(len(clientCtx.Data))},
		Data:    clientCtx.Data,
	}

	bytes, err := utils.EncodePacket(packet)
	if err != nil {
		fmt.Println(err)
		return
	}

	_, err = clientCtx.Socket.Write(bytes)
	if err != nil {
		fmt.Println(err)
		cleanup(clientCtx)
	}

	fmt.Printf("Sent -> %s with packet: %v", clientCtx.Data, packet)
	receive(clientCtx)
}

func waitForSynAck(clientCtx *ClientCtx) {
	buffer := make([]byte, 1024)

	n, _, err := clientCtx.Socket.ReadFromUDP(buffer)
	if err != nil {
		fmt.Println(err)
		cleanup(clientCtx)
	}

	bytes := buffer[0:n]

	packet, err := utils.DecodePacket(bytes)
	if err != nil {
		fmt.Println(err)
		cleanup(clientCtx)
	}

	clientCtx.Packet = packet

	if packet.Header.Flags.SYN && packet.Header.Flags.ACK {
		fmt.Println("Received -> SYN/ACK with packet: ", packet)
		sendAck(clientCtx)
	} else {
		fmt.Println("The packet wasn't a SYN/ACK packet")
	}
}

func sendAck(clientCtx *ClientCtx) {
	packet := utils.Packet{
		SrcAddr: clientCtx.Address,
		DstAddr: clientCtx.Socket.LocalAddr().String(),
		Header:  utils.Header{Flags: utils.Flags{ACK: true}, Seq: 1, Ack: 1, Len: 0},
	}

	bytes, err := utils.EncodePacket(packet)
	if err != nil {
		fmt.Println(err)
		return
	}

	_, err = clientCtx.Socket.Write(bytes)
	if err != nil {
		fmt.Println(err)
		cleanup(clientCtx)
	}
	fmt.Println("Sent -> ACK with packet: ", packet)
	send(clientCtx)
}

func sendSyn(clientCtx *ClientCtx) {
	packet := utils.Packet{
		SrcAddr: clientCtx.Address,
		DstAddr: clientCtx.Socket.LocalAddr().String(),
		Header:  utils.Header{Flags: utils.Flags{SYN: true}, Seq: 0, Ack: 0, Len: 0},
	}

	bytes, err := utils.EncodePacket(packet)
	if err != nil {
		fmt.Println(err)
		return
	}

	_, err = clientCtx.Socket.Write(bytes)
	if err != nil {
		fmt.Println(err)
		cleanup(clientCtx)
	}

	fmt.Println("Sent -> SYN with packet: ", clientCtx.Packet)
	waitForSynAck(clientCtx)
}

func readFile(clientCtx *ClientCtx) {
	content, err := os.ReadFile(clientCtx.FilePath)
	if err != nil {
		fmt.Println("Read File Error:\n", err)
	}

	if len(content) == 0 {
		fmt.Println("File is empty")
	}

	clientCtx.Data = string(content)
	sendSyn(clientCtx)
}

func bindSocket(clientCtx *ClientCtx) {
	s, _ := net.ResolveUDPAddr("udp4", clientCtx.Address)
	c, err := net.DialUDP("udp4", nil, s)
	if err != nil {
		fmt.Println(err)
		exit(clientCtx)
	}

	clientCtx.Socket = c

	fmt.Printf("The UDP server is %s\n", clientCtx.Socket.RemoteAddr().String())
	readFile(clientCtx)
}

func parseArgs(clientCtx *ClientCtx) {
	arguments := os.Args
	if len(arguments) == 1 {
		fmt.Println("Please provide a host:port string")
		exit(clientCtx)
	}
	clientCtx.Address = os.Args[1]
	clientCtx.FilePath = os.Args[2]

	bindSocket(clientCtx)
}

func main() {
	clientCtx := ClientCtx{}
	parseArgs(&clientCtx)
}
