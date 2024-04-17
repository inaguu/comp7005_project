package main

import (
	"comp7005_project/utils"
	"fmt"
	"math"
	"net"
	"os"
	"time"
)

const CLIENT_DELAY_SECONDS int = 10

type ClientCtx struct {
	Socket            *net.UDPConn
	Address, Ip, Port string
	FilePath          string
	Data              string

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

func sendSynPacket(clientCtx *ClientCtx) {
	sendPacket(clientCtx, utils.Flags{SYN: true}, "", 0, 0)
}

func sendAckPacket(clientCtx *ClientCtx) {
	lastPacketReceived := clientCtx.packetsReceived[len(clientCtx.packetsReceived)-1]
	lastPacketSent := clientCtx.packetsSent[len(clientCtx.packetsSent)-1]

	sendPacket(clientCtx, utils.Flags{ACK: true}, "", lastPacketReceived.Header.Ack, lastPacketSent.Header.Ack+lastPacketReceived.Header.Len)
}

func sendDataPacket(clientCtx *ClientCtx, data string) {
	lastPacketReceived := clientCtx.packetsReceived[len(clientCtx.packetsReceived)-1]
	lastPacketSent := clientCtx.packetsSent[len(clientCtx.packetsSent)-1]

	sendPacket(clientCtx, utils.Flags{PSH: true, ACK: true}, data, lastPacketReceived.Header.Ack, lastPacketSent.Header.Ack)
}

func sendFinPacket(clientCtx *ClientCtx) {
	lastPacketReceived := clientCtx.packetsReceived[len(clientCtx.packetsReceived)-1]
	lastPacketSent := clientCtx.packetsSent[len(clientCtx.packetsSent)-1]

	sendPacket(clientCtx, utils.Flags{FIN: true}, "", lastPacketReceived.Header.Ack, lastPacketSent.Header.Ack)
}

func sendLastPacket(clientCtx *ClientCtx) {
	lastPacketSent := clientCtx.packetsSent[len(clientCtx.packetsSent)-1]

	sendPacket(clientCtx, lastPacketSent.Header.Flags, lastPacketSent.Data, lastPacketSent.Header.Seq, lastPacketSent.Header.Ack)
}

func sendPacket(clientCtx *ClientCtx, flags utils.Flags, data string, seq uint32, ack uint32) {
	length := len(data)

	if length == 0 && (flags.SYN || flags.FIN) {
		length = 1
	}

	packet := utils.Packet{
		SrcAddr: clientCtx.Address,
		DstAddr: clientCtx.Socket.LocalAddr().String(),
		Header:  utils.Header{Flags: flags, Seq: seq, Ack: ack, Len: uint32(length)},
		Data:    data,
	}

	bytes, err := utils.EncodePacket(packet)
	if err != nil {
		fmt.Println(err)
		cleanup(clientCtx)
	}

	_, err = clientCtx.Socket.Write(bytes)
	if err != nil {
		fmt.Println(err)
		cleanup(clientCtx)
	}

	clientCtx.packetsSent = append(clientCtx.packetsSent, packet)
}

func flagsMatch(flags1, flags2 utils.Flags) bool {
	return (flags1.SYN == flags2.SYN) && (flags1.FIN == flags2.FIN) && (flags1.ACK == flags2.ACK) && (flags1.PSH == flags2.PSH)
}

// checks if the flags are as expected, false if no or timeout when receiving
func hasReceivedPacket(clientCtx *ClientCtx, flags utils.Flags) bool {
	buffer := make([]byte, 1024)

	deadline := time.Now().Add(time.Duration(CLIENT_DELAY_SECONDS) * time.Second)
	clientCtx.Socket.SetReadDeadline(deadline)

	n, _, err := clientCtx.Socket.ReadFromUDP(buffer)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
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

	if packet.Header.Flags.ACK && (!packet.Header.Flags.FIN || packet.Header.Flags.SYN) {
		return flagsMatch(flags, packet.Header.Flags) && packet.Header.Ack > lastPacketSent.Header.Seq
	}
	return flagsMatch(flags, packet.Header.Flags)
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

func correctAck(clientCtx *ClientCtx) bool {
	lastPacketReceieved := clientCtx.packetsReceived[len(clientCtx.packetsReceived)-1]
	lastPacketSent := clientCtx.packetsSent[len(clientCtx.packetsSent)-1]

	return lastPacketReceieved.Header.Ack > lastPacketSent.Header.Seq
}

func send(clientCtx *ClientCtx) {
	ackFlag := utils.Flags{ACK: true}

	chunkSize := 512

	for i := 0; i < len(clientCtx.Data); i += chunkSize {
		chunkEnd := math.Min(float64(len(clientCtx.Data)), float64(i+chunkSize))
		chunk := clientCtx.Data[i:int(chunkEnd)]

		sendDataPacket(clientCtx, chunk)
		lastPacketSent := clientCtx.packetsSent[len(clientCtx.packetsSent)-1]
		fmt.Println("Sent -> PSH/ACK:", packetString(lastPacketSent))

		for !hasReceivedPacket(clientCtx, ackFlag) {
			fmt.Println("Timeout waiting for ACK")
			sendLastPacket(clientCtx)

			lastPacketSent = clientCtx.packetsSent[len(clientCtx.packetsSent)-1]
			fmt.Println("Sent -> REPEAT PSH/ACK:", packetString(lastPacketSent))
		}
		lastPacketReceieved := clientCtx.packetsReceived[len(clientCtx.packetsReceived)-1]
		fmt.Println("Received -> ACK:", packetString(lastPacketReceieved))
	}
}

func readFile(clientCtx *ClientCtx) {
	content, err := os.ReadFile(clientCtx.FilePath)
	if err != nil {
		fmt.Println("Read File Error:\n", err)
	}

	if len(content) == 0 {
		fmt.Println("File is empty")
		cleanup(clientCtx)
	}

	clientCtx.Data = string(content)
}

func bindSocket(clientCtx *ClientCtx) {
	s, _ := net.ResolveUDPAddr("udp", utils.Address(clientCtx.Ip, clientCtx.Port))
	c, err := net.DialUDP("udp", nil, s)
	if err != nil {
		fmt.Println(err)
		exit(clientCtx)
	}

	clientCtx.Socket = c

	fmt.Printf("The UDP server is %s\n", clientCtx.Socket.RemoteAddr().String())
}

func terminateConnection(clientCtx *ClientCtx) {
	sendFinPacket(clientCtx)
	lastPacketSent := clientCtx.packetsSent[len(clientCtx.packetsSent)-1]
	fmt.Println("Sent -> FIN:", packetString(lastPacketSent))

	finAckFlags := utils.Flags{FIN: true, ACK: true}
	for !hasReceivedPacket(clientCtx, finAckFlags) {
		fmt.Println("Timeout waiting for FIN/ACK")
		sendLastPacket(clientCtx)
		lastPacketSent = clientCtx.packetsSent[len(clientCtx.packetsSent)-1]
		fmt.Println("Sent -> REPEAT FIN:", packetString(lastPacketSent))
	}
	lastPacketReceieved := clientCtx.packetsReceived[len(clientCtx.packetsReceived)-1]
	fmt.Println("Received -> FIN/ACK:", packetString(lastPacketReceieved))

	sendAckPacket(clientCtx)
	lastPacketSent = clientCtx.packetsSent[len(clientCtx.packetsSent)-1]
	fmt.Println("Sent -> ACK:", packetString(lastPacketSent))

	// if server sends fin/ack again, they did not get the final ack
	for hasReceivedPacket(clientCtx, finAckFlags) {
		sendLastPacket(clientCtx)
		lastPacketSent = clientCtx.packetsSent[len(clientCtx.packetsSent)-1]
		fmt.Println("Sent -> REPEAT ACK:", packetString(lastPacketSent))
	}
}

func establishConnection(clientCtx *ClientCtx) {
	sendSynPacket(clientCtx)
	lastPacketSent := clientCtx.packetsSent[len(clientCtx.packetsSent)-1]
	fmt.Println("Sent -> SYN:", packetString(lastPacketSent))

	synAckFlags := utils.Flags{SYN: true, ACK: true}
	for !hasReceivedPacket(clientCtx, synAckFlags) {
		fmt.Println("Timeout waiting for SYN/ACK")
		sendLastPacket(clientCtx)
		lastPacketSent = clientCtx.packetsSent[len(clientCtx.packetsSent)-1]
		fmt.Println("Sent -> REPEAT SYN:", packetString(lastPacketSent))
	}
	lastPacketReceieved := clientCtx.packetsReceived[len(clientCtx.packetsReceived)-1]
	fmt.Println("Received -> SYN/ACK:", packetString(lastPacketReceieved), lastPacketReceieved.Header.Len)

	sendAckPacket(clientCtx)
	lastPacketSent = clientCtx.packetsSent[len(clientCtx.packetsSent)-1]
	fmt.Println("Sent -> ACK:", packetString(lastPacketSent))

	// if server sends syn/ack again, they did not get the final ack
	for hasReceivedPacket(clientCtx, synAckFlags) {
		sendLastPacket(clientCtx)
		lastPacketSent = clientCtx.packetsSent[len(clientCtx.packetsSent)-1]
		fmt.Println("Sent -> REPEAT ACK:", packetString(lastPacketSent))
	}
	fmt.Println("Connection Established")
}

func checkArgs(clientCtx *ClientCtx) {
	address := utils.Address(clientCtx.Ip, clientCtx.Port)
	if address == "" {
		fmt.Printf("%s and %s is not a valid ip and port combination\n", clientCtx.Ip, clientCtx.Port)
		exit(clientCtx)
	}
	clientCtx.Address = address
}

func parseArgs(clientCtx *ClientCtx) {
	arguments := os.Args
	if len(arguments) < 4 {
		fmt.Println("Not enough arguments")
		exit(clientCtx)
	}

	clientCtx.Ip = os.Args[1]
	clientCtx.Port = os.Args[2]
	clientCtx.FilePath = os.Args[3]

	checkArgs(clientCtx)
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
