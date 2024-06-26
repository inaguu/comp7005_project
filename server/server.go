package main

import (
	"comp7005_project/utils"
	"fmt"
	"math/rand"
	"net"
	"os"
	"time"
)

const SERVER_DELAY_SECONDS int = 1

type ServerCtx struct {
	Socket        *net.UDPConn
	ClientAddress *net.UDPAddr

	packetsSent, packetsReceived []utils.Packet

	Ip, Port string
	Packet   utils.Packet

	Timeout bool

	EstablishCount   int
	TerminationCount int
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
		Header:  utils.Header{Flags: utils.Flags{FIN: true, ACK: true}, Seq: serverCtx.Packet.Header.Ack, Ack: serverCtx.Packet.Header.Seq + serverCtx.Packet.Header.Len, Len: 0},
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
	fmt.Println("Send -> FIN/ACK with packet:", packetString(packet))

	waitForAck(serverCtx)
}

func send(serverCtx *ServerCtx) {
	lastPacketReceived := serverCtx.packetsReceived[len(serverCtx.packetsReceived)-1]
	// lastPacketSent := serverCtx.packetsSent[len(serverCtx.packetsSent)-1]

	packet := utils.Packet{
		SrcAddr: serverCtx.Packet.SrcAddr,
		DstAddr: serverCtx.Packet.DstAddr,
		Header:  utils.Header{Flags: utils.Flags{ACK: true}, Seq: lastPacketReceived.Header.Ack, Ack: lastPacketReceived.Header.Seq + lastPacketReceived.Header.Len, Len: 1},
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

	if serverCtx.Timeout {
		deadline := time.Now().Add(time.Duration(SERVER_DELAY_SECONDS) * time.Second)
		serverCtx.Socket.SetReadDeadline(deadline)
	}

	n, addr, err := serverCtx.Socket.ReadFromUDP(buffer)
	if err != nil {
		if serverCtx.Timeout {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				fmt.Println("Timeout waiting for PSH/ACK")
				sendLastPacket(serverCtx)
				// receive(serverCtx)
			} else {
				fmt.Println(err)
				cleanup(serverCtx)
			}
		} else {
			fmt.Println(err)
			cleanup(serverCtx)
		}
	}

	bytes := buffer[0:n]
	var packet utils.Packet

	packet, err = utils.DecodePacket(bytes)
	if err != nil {
		fmt.Println(err)
		cleanup(serverCtx)
	}

	// if len(bytes) != 0 {
	// 	packet, err = utils.DecodePacket(bytes)
	// 	if packet.Header.Flags.ACK && !packet.Header.Flags.PSH && !serverCtx.Timeout {
	// 		receive(serverCtx)
	// 	}
	// 	if err != nil {
	// 		fmt.Println(err)
	// 		cleanup(serverCtx)
	// 	}
	// } else {
	// 	receive(serverCtx)
	// }

	serverCtx.packetsReceived = append(serverCtx.packetsReceived, packet)
	serverCtx.ClientAddress = addr
	serverCtx.Packet = packet

	if packet.Header.Flags.SYN {
		fmt.Println("Received -> SYN with packet:", packetString(packet))
		sendSynAck(serverCtx)
	} else if packet.Header.Flags.FIN {
		fmt.Println("Received -> FIN with packet:", packetString(packet))
		sendFinAck(serverCtx)
	} else if packet.Header.Flags.PSH && packet.Header.Flags.ACK {
		fmt.Printf("Received -> %s with packet: %s", "packet.Data", packetString(packet))
		serverCtx.Timeout = true
		send(serverCtx)
	}
	receive(serverCtx)
}

func sendLastPacket(serverCtx *ServerCtx) {
	lastPacketSent := serverCtx.packetsSent[len(serverCtx.packetsSent)-1]

	lastPacketSent.Header.Flags.DUP = true

	bytes, err := utils.EncodePacket(lastPacketSent)
	if err != nil {
		fmt.Println(err)
		cleanup(serverCtx)
	}

	_, err = serverCtx.Socket.WriteToUDP(bytes, serverCtx.ClientAddress)
	if err != nil {
		fmt.Println(err)
		cleanup(serverCtx)
	}

	serverCtx.packetsSent = append(serverCtx.packetsSent, lastPacketSent)

	if lastPacketSent.Header.Flags.ACK && lastPacketSent.Header.Flags.FIN {
		fmt.Println("Re-Send -> FIN/ACK with packet: ", packetString(lastPacketSent))
		serverCtx.TerminationCount++
		if serverCtx.TerminationCount >= 7 {
			fmt.Println("Passed FIN/ACK resending limit")
			fmt.Println("Connection terminated")
			serverCtx.Timeout = false
			serverCtx.Socket.SetReadDeadline(time.Time{})
			receive(serverCtx)
		}
		waitForAck(serverCtx)
	} else if lastPacketSent.Header.Flags.ACK && lastPacketSent.Header.Flags.SYN {
		fmt.Println("Re-Send -> SYN/ACK with packet: ", packetString(lastPacketSent))
		serverCtx.EstablishCount++
		if serverCtx.EstablishCount >= 7 {
			fmt.Println("Passed SYN/ACK resending limit")
			fmt.Println("Connection terminated")
			serverCtx.Timeout = false
			serverCtx.Socket.SetReadDeadline(time.Time{})
			receive(serverCtx)
		}
		waitForAck(serverCtx)
	} else {
		fmt.Println("Re-Send -> ACK with packet: ", packetString(lastPacketSent))
		receive(serverCtx)
	}
}

func sendSynAck(serverCtx *ServerCtx) {
	packet := utils.Packet{
		SrcAddr: serverCtx.Packet.SrcAddr,
		DstAddr: serverCtx.Packet.DstAddr,
		Header:  utils.Header{Flags: utils.Flags{SYN: true, ACK: true}, Seq: 0, Ack: 1, Len: 1},
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
	fmt.Println("Send -> SYN/ACK with packet:", packetString(packet))
	waitForAck(serverCtx)
}

func waitForAck(serverCtx *ServerCtx) {
	buffer := make([]byte, 1024)

	deadline := time.Now().Add(time.Duration(SERVER_DELAY_SECONDS) * time.Second)
	serverCtx.Socket.SetReadDeadline(deadline)

	n, _, err := serverCtx.Socket.ReadFromUDP(buffer)
	if err != nil {
		if netError, ok := err.(net.Error); ok && netError.Timeout() {
			fmt.Println("Timeout waiting for ACK -> re-sending packet")
			sendLastPacket(serverCtx)
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

	lastPacketReceived := serverCtx.packetsReceived[len(serverCtx.packetsReceived)-1]
	serverCtx.packetsReceived = append(serverCtx.packetsReceived, packet)

	if packet.Header.Flags.ACK && lastPacketReceived.Header.Flags.SYN {
		fmt.Println("Received -> ACK with packet:", packetString(packet))
		fmt.Println("Connection established")
		serverCtx.Timeout = false
		serverCtx.Socket.SetReadDeadline(time.Time{})
		receive(serverCtx)
	} else if packet.Header.Flags.ACK && lastPacketReceived.Header.Flags.FIN {
		fmt.Println("Received -> ACK with packet:", packetString(packet))
		fmt.Println("Connection terminated")
		serverCtx.Timeout = false
		serverCtx.Socket.SetReadDeadline(time.Time{})
		receive(serverCtx)
	} else {
		fmt.Println("The packet wasn't an ACK packet")
		waitForAck(serverCtx)
	}
}

func bindSocket(serverCtx *ServerCtx) {
	s, err := net.ResolveUDPAddr("udp", utils.Address(serverCtx.Ip, serverCtx.Port))
	if err != nil {
		fmt.Println(err)
		exit(serverCtx)
	}

	connection, err := net.ListenUDP("udp", s)
	if err != nil {
		fmt.Println(err)
		exit(serverCtx)
	}

	serverCtx.Socket = connection
}

func checkArgs(serverCtx *ServerCtx) {
	if utils.Address(serverCtx.Ip, serverCtx.Port) == "" {
		fmt.Printf("%s and %s is not a valid ip and port combination", serverCtx.Ip, serverCtx.Port)
		exit(serverCtx)
	}
}

func parseArgs(serverCtx *ServerCtx) {
	if len(os.Args) < 3 {
		fmt.Println(INPUT_ERROR)
		exit(serverCtx)
	}

	serverCtx.Ip = os.Args[1]
	serverCtx.Port = os.Args[2]

	checkArgs(serverCtx)

	fmt.Printf("The UDP server is %s\n", utils.Address(serverCtx.Ip, serverCtx.Port))
}

func main() {
	serverCtx := ServerCtx{}
	parseArgs(&serverCtx)
	bindSocket(&serverCtx)
	receive(&serverCtx)
}
