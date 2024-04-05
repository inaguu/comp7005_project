package main

import (
	"fmt"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	INPUT_ERROR = "Usage: <filename> -i <ip address> -p <port_number>"
)

var (
	IP         string
	port       string
	addr       *net.UDPAddr
	connection *net.UDPConn
)

func random(min, max int) int {
	return rand.Intn(max-min) + min
}

func initServer(port string) {
	PORT := ":" + port
	addr, err := net.ResolveUDPAddr("udp4", PORT)
	if err != nil {
		fmt.Println(err)
		return
	}

	connection, err = net.ListenUDP("udp", addr)
	if err != nil {
		fmt.Println(err)
		return
	}
}

func parseArgs(args []string) {
	if len(args) == 1 {
		fmt.Println(INPUT_ERROR)
		return
	}
	fmt.Println(args)
}

func main() {

	parseArgs(os.Args)

	arguments := os.Args
	if len(arguments) == 1 {
		fmt.Println("Please provide a port number!")
		return
	}
	// PORT := ":" + arguments[1]

	s, err := net.ResolveUDPAddr("udp4", "127.0.0.1:1234")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(s)

	connection, err := net.ListenUDP("udp4", s)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(connection)

	defer connection.Close()
	buffer := make([]byte, 1024)
	rand.Seed(time.Now().Unix())

	for {
		n, addr, err := connection.ReadFromUDP(buffer)
		fmt.Print("-> ", string(buffer[0:n-1]))

		if strings.TrimSpace(string(buffer[0:n])) == "STOP" {
			fmt.Println("Exiting UDP server!")
			return
		}

		data := []byte(strconv.Itoa(random(1, 1001)))
		fmt.Printf("data: %s\n", string(data))
		_, err = connection.WriteToUDP(data, addr)
		if err != nil {
			fmt.Println(err)
			return
		}
	}
}
