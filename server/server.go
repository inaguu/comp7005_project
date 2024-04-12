package main

import (
	"bytes"
	"encoding/binary"
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
}

const (
	INPUT_ERROR     = "Usage: <filename> <ip address> <port_number>"
	ServerKey   Key = 0
)

type packet struct {
	SYN uint8
	ACK uint8
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

func send(serverCtx *ServerCtx) {
	rand.Seed(time.Now().Unix())

	data := []byte(strconv.Itoa(random(1, 1001)))
	fmt.Printf("\ndata: %s\n", string(data))

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

	fmt.Print("-> ", string(buffer[0:n]))

	serverCtx.ClientAddress = addr

	send(serverCtx)
}

func synReceive(serverCtx *ServerCtx) {
	buffer := make([]byte, 1024)

	n, _, err := serverCtx.Socket.ReadFromUDP(buffer)
	if err != nil {
		fmt.Println(err)
		cleanup(serverCtx)
	}

	buf := bytes.NewBuffer(buffer[0:n])
	var synPacket packet

	errPacket := binary.Read(buf, binary.BigEndian, &synPacket)
	if errPacket != nil {
		fmt.Println("failed to Read:", errPacket)
		return
	}

	fmt.Printf("-> %v\n", synPacket)

	waitForAck(serverCtx)
}

func waitForAck(serverCtx *ServerCtx) {
	receive(serverCtx)
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

	synReceive(serverCtx)
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
