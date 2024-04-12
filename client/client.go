package main

import (
	"comp7005_project/utils"
	"fmt"
	"net"
	"os"
	"time"
)

type Key int

const ClientKey Key = 0

type ClientCtx struct {
	Socket     *net.UDPConn
	Address    string
	FilePath   string
	Data       string
	Packet     utils.Packet
	DataToSend []string
}

func buildPackets(clientCtx *ClientCtx) []utils.Packet {
	var packets []utils.Packet

	for i := range clientCtx.DataToSend {
		packet := utils.Packet{
			SrcAddr: clientCtx.Address,
			DstAddr: clientCtx.Socket.LocalAddr().String(),
			Header:  utils.Header{Flags: utils.Flags{PSH: true, ACK: true}, Seq: clientCtx.Packet.Header.Ack, Ack: clientCtx.Packet.Header.Seq + clientCtx.Packet.Header.Len + 1, Len: uint32(len(clientCtx.Data))},
			Data:    clientCtx.DataToSend[i],
		}

		packets = append(packets, packet)
	}

	return packets
}

func packetString(packet utils.Packet) string {
	return fmt.Sprintf("[Seq: %d | Ack: %d]", packet.Header.Seq, packet.Header.Ack)
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
	fmt.Println("Sent -> ACK:", packetString(packet))

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
		fmt.Println("Received -> FIN/ACK with packet:", packetString(packet))
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
	fmt.Println("Sent -> FIN with packet:", packetString(packet))
	waitForFinAck(clientCtx)
}

func receive(clientCtx *ClientCtx) bool {
	buffer := make([]byte, 1024)

	deadline := time.Now().Add(10 * time.Second)
	clientCtx.Socket.SetWriteDeadline(deadline)

	n, _, err := clientCtx.Socket.ReadFromUDP(buffer)
	if err != nil {
		fmt.Println(err)
		cleanup(clientCtx)
	}

	bytes := buffer[0:n]

	packet, err := utils.DecodePacket(bytes)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			fmt.Println("Timeout")
			return false
		} else {
			fmt.Println(err)
			cleanup(clientCtx)
		}
	}

	clientCtx.Packet = packet

	if clientCtx.Packet.Header.Flags.ACK {
		fmt.Println("\nReceived -> ACK with packet:", packetString(packet))
		return true
	} else {
		fmt.Println("\nDid not receive ACK packet")
	}
	return false
}

func splitData(clientCtx *ClientCtx) {
	if (len(clientCtx.Data)) > 512 {
		clientCtx.DataToSend = append(clientCtx.DataToSend, clientCtx.Data[0:512])
		clientCtx.DataToSend = append(clientCtx.DataToSend, clientCtx.Data[512:1024])
	}
}

func send(clientCtx *ClientCtx) {
	splitData(clientCtx)

	packets := buildPackets(clientCtx)

	for _, packet := range packets {
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

		fmt.Printf("Sent -> %s with packet: %s", clientCtx.Data, packetString(packet))

		for !receive(clientCtx) {
			_, err = clientCtx.Socket.Write(bytes)
			if err != nil {
				fmt.Println(err)
				cleanup(clientCtx)
			}
		}
	}

	sendFin(clientCtx)
}

func synAckReceived(clientCtx *ClientCtx) bool {
	buffer := make([]byte, 1024)

	deadline := time.Now().Add(10 * time.Second)
	clientCtx.Socket.SetReadDeadline(deadline)

	n, _, err := clientCtx.Socket.ReadFromUDP(buffer)
	if err != nil {
		if netError, ok := err.(net.Error); ok && netError.Timeout() {
			fmt.Println("Timeout waiting for SYN/ACK")
			return false
		} else {
			fmt.Println(err)
			cleanup(clientCtx)
		}
	}

	bytes := buffer[0:n]

	packet, err := utils.DecodePacket(bytes)
	if err != nil {
		fmt.Println(err)
		cleanup(clientCtx)
	}

	clientCtx.Packet = packet

	if packet.Header.Flags.SYN && packet.Header.Flags.ACK {
		fmt.Println("Received -> SYN/ACK with packet:", packetString(packet))
		return true
	} else {
		fmt.Println("The packet wasn't a SYN/ACK packet")
	}
	return false
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
	fmt.Println("Sent -> ACK with packet:", packetString(packet))
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

	fmt.Println("Sent -> SYN with packet:", packetString(packet))
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
	establishConnection(clientCtx)
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

func terminateConnection() {}

func establishConnection(clientCtx *ClientCtx) {
	sendSyn(clientCtx)
	for !synAckReceived(clientCtx) {
		sendSyn(clientCtx)
	}

	sendAck(clientCtx)
	send(clientCtx)
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
