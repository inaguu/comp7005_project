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
}

type packet struct {
	SYN uint8
	ACK uint8
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

func receive(clientCtx *ClientCtx) {
	buffer := make([]byte, 1024)

	n, _, err := clientCtx.Socket.ReadFromUDP(buffer)
	if err != nil {
		fmt.Println(err)
		cleanup(clientCtx)
	}

	fmt.Printf("Reply: %s\n", string(buffer[0:n]))

	cleanup(clientCtx)
}

func send(clientCtx *ClientCtx) {
	_, err := clientCtx.Socket.Write([]byte(clientCtx.Data))
	if err != nil {
		fmt.Println(err)
		cleanup(clientCtx)
	}
	fmt.Println("sent", clientCtx.Data)
	receive(clientCtx)
}

func waitForSynAck(clientCtx *ClientCtx) {
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
