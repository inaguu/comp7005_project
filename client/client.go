package main

import (
	"comp7005_project/utils"
	"fmt"
	"math"
	"net"
	"os"
	"time"
)

type ClientCtx struct {
	Socket   *net.UDPConn
	Address  string
	FilePath string
	Data     string

	DataToSend                   []string
	packetsSent, packetsReceived []utils.Packet
}

func buildPackets(clientCtx *ClientCtx) []utils.Packet {
	var packets []utils.Packet

	chunkSize := 512

	for i := 0; i < len(clientCtx.Data); i += chunkSize {
		chunkEnd := math.Min(float64(len(clientCtx.Data)), float64(i+chunkSize))
		chunk := clientCtx.Data[i:int(chunkEnd)]

		packet := utils.Packet{
			SrcAddr: clientCtx.Address,
			DstAddr: clientCtx.Socket.LocalAddr().String(),
			Header:  utils.Header{Flags: utils.Flags{PSH: true, ACK: true}, Seq: 0, Ack: 0, Len: uint32(len(chunk))},
			Data:    chunk,
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
	lastReceivedPacket := clientCtx.packetsReceived[len(clientCtx.packetsReceived)-1]
	lastSentPacket := clientCtx.packetsSent[len(clientCtx.packetsSent)-1]

	packet := utils.Packet{
		SrcAddr: clientCtx.Address,
		DstAddr: clientCtx.Socket.LocalAddr().String(),
		Header:  utils.Header{Flags: utils.Flags{ACK: true}, Seq: lastReceivedPacket.Header.Ack, Ack: lastSentPacket.Header.Ack + lastReceivedPacket.Header.Len, Len: 1},
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

	clientCtx.packetsSent = append(clientCtx.packetsSent, packet)
	fmt.Println("Sent -> ACK:", packetString(packet))

	cleanup(clientCtx)
}

func waitForFinAck(clientCtx *ClientCtx) bool {
	buffer := make([]byte, 1024)

	deadline := time.Now().Add(10 * time.Second)
	clientCtx.Socket.SetReadDeadline(deadline)

	n, _, err := clientCtx.Socket.ReadFromUDP(buffer)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			fmt.Println("Timeout waiting for FIN/ACK")
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

	clientCtx.packetsReceived = append(clientCtx.packetsReceived, packet)

	if packet.Header.Flags.FIN && packet.Header.Flags.ACK {
		fmt.Println("Received -> FIN/ACK with packet:", packetString(packet))
		return true
	} else {
		fmt.Println("The packet wasn't a FIN/ACK")
	}
	return false
}

func sendFin(clientCtx *ClientCtx) {
	lastReceivedPacket := clientCtx.packetsReceived[len(clientCtx.packetsReceived)-1]
	lastSentPacket := clientCtx.packetsSent[len(clientCtx.packetsSent)-1]

	packet := utils.Packet{
		SrcAddr: clientCtx.Address,
		DstAddr: clientCtx.Socket.LocalAddr().String(),
		Header:  utils.Header{Flags: utils.Flags{FIN: true}, Seq: lastReceivedPacket.Header.Ack, Ack: lastSentPacket.Header.Ack, Len: 1},
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
	clientCtx.packetsSent = append(clientCtx.packetsSent, packet)
	fmt.Println("Sent -> FIN with packet:", packetString(packet))
}

func receive(clientCtx *ClientCtx) bool {
	buffer := make([]byte, 1024)

	deadline := time.Now().Add(10 * time.Second)
	clientCtx.Socket.SetReadDeadline(deadline)

	n, _, err := clientCtx.Socket.ReadFromUDP(buffer)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			fmt.Println("Timeout")
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

	clientCtx.packetsReceived = append(clientCtx.packetsReceived, packet)

	lastPacketSent := clientCtx.packetsSent[len(clientCtx.packetsSent)-1]

	if packet.Header.Flags.ACK {
		fmt.Println("\nReceived -> ACK with packet:", packetString(packet))
		// if server got packet the ack number should have increased
		return packet.Header.Ack > lastPacketSent.Header.Seq
	} else {
		fmt.Println("\nDid not receive ACK packet")
	}
	return false
}

func send(clientCtx *ClientCtx) {
	packets := buildPackets(clientCtx)

	for _, packet := range packets {
		lastReceivedPacket := clientCtx.packetsReceived[len(clientCtx.packetsReceived)-1]
		lastSentPacket := clientCtx.packetsSent[len(clientCtx.packetsSent)-1]
		packet.Header.Seq = lastReceivedPacket.Header.Ack
		packet.Header.Ack = lastSentPacket.Header.Ack + lastReceivedPacket.Header.Len

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

		clientCtx.packetsSent = append(clientCtx.packetsSent, packet)
		fmt.Printf("Sent -> %s with packet: %s", "No REPEAT", packetString(packet))

		for !receive(clientCtx) {
			_, err = clientCtx.Socket.Write(bytes)
			if err != nil {
				fmt.Println(err)
				cleanup(clientCtx)
			}
			fmt.Printf("Sent -> %s with packet: %s", "REPEAT", packetString(packet))
		}
	}
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

	clientCtx.packetsReceived = append(clientCtx.packetsReceived, packet)

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

	clientCtx.packetsSent = append(clientCtx.packetsSent, packet)
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

	clientCtx.packetsSent = append(clientCtx.packetsSent, packet)
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
}

func terminateConnection(clientCtx *ClientCtx) {
	sendFin(clientCtx)
	for !waitForFinAck(clientCtx) {
		sendFin(clientCtx)
	}
	sendFinalAck(clientCtx)
}

func establishConnection(clientCtx *ClientCtx) {
	sendSyn(clientCtx)
	for !synAckReceived(clientCtx) {
		sendSyn(clientCtx)
	}

	sendAck(clientCtx)
}

func parseArgs(clientCtx *ClientCtx) {
	arguments := os.Args
	if len(arguments) == 1 {
		fmt.Println("Please provide a host:port string")
		exit(clientCtx)
	}
	clientCtx.Address = os.Args[1]
	clientCtx.FilePath = os.Args[2]
}

func main() {
	clientCtx := ClientCtx{}
	parseArgs(&clientCtx)
	bindSocket(&clientCtx)
	readFile(&clientCtx)
	establishConnection(&clientCtx)
	send(&clientCtx)
	terminateConnection(&clientCtx)
}
