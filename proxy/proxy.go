package main

import (
	"comp7005_project/utils"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"os"
	"time"
)

type ProxyCtx struct {
	Socket                 *net.UDPConn
	ClientAddress          *net.UDPAddr
	ServerAddress          *net.UDPAddr
	ProxyAddress           *net.UDPAddr
	SIp, DIp, SPort, DPort string
	Data                   []byte

	ClientDropChance, ServerDropChance   int
	ClientDelayChance, ServerDelayChance int
	ClientDelayMin, ClientDelayMax       int
	ServerDelayMin, ServerDelayMax       int
}

func packetString(packet utils.Packet) string {
	return fmt.Sprintf("[Seq: %d | Ack: %d | Len: %d]", packet.Header.Seq, packet.Header.Ack, packet.Header.Len)
}

func exit(proxyCtx *ProxyCtx) {
	fmt.Println("Exiting...")
	os.Exit(0)
}

func cleanup(proxyCtx *ProxyCtx) {
	if proxyCtx.Socket != nil {
		proxyCtx.Socket.Close()
	}

	exit(proxyCtx)
}

func receive(proxyCtx *ProxyCtx) {
	buffer := make([]byte, 1024)
	n, addr, err := proxyCtx.Socket.ReadFromUDP(buffer)
	if err != nil {
		fmt.Println(err)
	}
	proxyCtx.Data = buffer[0:n]

	proxyCtx.ClientAddress = addr

	packet, _ := utils.DecodePacket(proxyCtx.Data)

	if sendTo(fmt.Sprintf("%s:%d", addr.IP, addr.Port), fmt.Sprintf("%s:%d", proxyCtx.ServerAddress.IP, proxyCtx.ServerAddress.Port)) {
		dropChance := rand.Intn(100)

		if dropChance < proxyCtx.ServerDropChance {
			fmt.Println("Packet dropped from server:", packetString(packet))
			receive(proxyCtx)
		}

		delayChance := rand.Intn(100)
		if delayChance < proxyCtx.ServerDelayChance {
			fmt.Println("Packet Delayed from server:", packetString(packet))
			go func() {
				time.Sleep(5 * time.Second)
				sendToClient(proxyCtx)
			}()
			receive(proxyCtx)
		} else {
			sendToClient(proxyCtx)
		}

	} else {
		dropChance := rand.Intn(100)

		if dropChance < proxyCtx.ClientDropChance {
			fmt.Println("Packet Dropped from client:", packetString(packet))
			receive(proxyCtx)
		}

		delayChance := rand.Intn(100)
		if delayChance < proxyCtx.ClientDelayChance {
			fmt.Println("Packet Delayed from client:", packetString(packet))
			go func() {
				time.Sleep(5 * time.Second)
				sendToServer(proxyCtx)
			}()
			receive(proxyCtx)
		} else {
			sendToServer(proxyCtx)
		}
	}
}

func sendTo(ip string, server string) bool {
	if ip == server {
		return true
	} else {
		return false
	}
}

func sendToClient(proxyCtx *ProxyCtx) {
	packet, err := utils.DecodePacket(proxyCtx.Data)
	if err != nil {
		fmt.Println(err)
		cleanup(proxyCtx)
	}

	addr, err := net.ResolveUDPAddr("udp", packet.DstAddr)
	if err != nil {
		fmt.Println(err)
		cleanup(proxyCtx)
	}

	_, err = proxyCtx.Socket.WriteToUDP([]byte(proxyCtx.Data), addr)
	if err != nil {
		fmt.Println(err)
		cleanup(proxyCtx)
	}
	receive(proxyCtx)
}

func sendToServer(proxyCtx *ProxyCtx) {
	_, err := proxyCtx.Socket.WriteToUDP([]byte(proxyCtx.Data), proxyCtx.ServerAddress)
	if err != nil {
		fmt.Println(err)
		cleanup(proxyCtx)
	}
	receive(proxyCtx)
}

func connectToServer(proxyCtx *ProxyCtx) {
	s, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%s", proxyCtx.DIp, proxyCtx.DPort))
	if err != nil {
		fmt.Println(err)
		exit(proxyCtx)
	}

	proxyCtx.ServerAddress = s
	_, err = net.DialUDP("udp4", nil, s)
	if err != nil {
		cleanup(proxyCtx)
	}

	fmt.Println("Connected to UDP server at", proxyCtx.ServerAddress)

	receive(proxyCtx)
}

// both
func bind_socket(proxyCtx *ProxyCtx) {
	s, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%s", proxyCtx.SIp, proxyCtx.SPort))
	if err != nil {
		fmt.Println(err)
		exit(proxyCtx)
	}

	connection, err := net.ListenUDP("udp4", s)
	if err != nil {
		fmt.Println(err)
		exit(proxyCtx)
	}

	proxyCtx.ProxyAddress = s
	proxyCtx.Socket = connection

	fmt.Println("The proxy UDP server is", proxyCtx.ProxyAddress)

	connectToServer(proxyCtx)
}

func usage() {
	fmt.Fprintf(flag.CommandLine.Output(), "Proxy is for simulating network unreliabilities\n\n")

	fmt.Fprintf(flag.CommandLine.Output(), "Usage:\n\n")

	fmt.Fprintf(flag.CommandLine.Output(), "  go run proxy/proxy.go [flags] <proxy ip> <proxy port> <server ip> <server port>\n\n")

	fmt.Fprintf(flag.CommandLine.Output(), "Arguments:\n\n")
	fmt.Fprintf(flag.CommandLine.Output(), "  proxy ip\n")
	fmt.Fprintf(flag.CommandLine.Output(), "\tip address for proxy to bind to\n")

	fmt.Fprintf(flag.CommandLine.Output(), "  proxy port\n")
	fmt.Fprintf(flag.CommandLine.Output(), "\tport for proxy to bind to\n")

	fmt.Fprintf(flag.CommandLine.Output(), "  server ip\n")
	fmt.Fprintf(flag.CommandLine.Output(), "\tip address of the running server\n")

	fmt.Fprintf(flag.CommandLine.Output(), "  server port\n")
	fmt.Fprintf(flag.CommandLine.Output(), "\tport of the running server\n")

	fmt.Fprintf(flag.CommandLine.Output(), "Flags:\n\n")
	flag.PrintDefaults()
}

func checkArgs(proxyCtx *ProxyCtx) {
	errorString := ""
	if proxyCtx.ClientDropChance < 0 || proxyCtx.ClientDropChance > 100 {
		errorString = "-cdrop must be an integer from 0 to 100"
	} else if proxyCtx.ServerDropChance < 0 || proxyCtx.ServerDropChance > 100 {
		errorString = "-sdrop must be an integer from 0 to 100"
	} else if proxyCtx.ClientDelayChance < 0 || proxyCtx.ClientDelayChance > 100 {
		errorString = "-cdelay must be an integer from 0 to 100"
	} else if proxyCtx.ServerDelayChance < 0 || proxyCtx.ServerDelayChance > 100 {
		errorString = "-sdelay must be an integer from 0 to 100"
	} else if proxyCtx.ClientDelayMin > proxyCtx.ClientDelayMax {
		errorString = "-cmin must be less than or equal to -cmax"
	} else if proxyCtx.ServerDelayMin > proxyCtx.ServerDelayMax {
		errorString = "-smin must be less than or equal to -smax"
	}

	if errorString != "" {
		fmt.Fprintf(flag.CommandLine.Output(), "%s\n", errorString)
		usage()
		exit(proxyCtx)
	}
}

func parseArgs(proxyCtx *ProxyCtx) {
	clientDropChance := flag.Int("cdrop", 0, "% drop chance for packets coming from client (0 - 100)")
	serverDropChance := flag.Int("sdrop", 0, "% drop chance for packets comming from server (0 - 100)")

	clientDelayChance := flag.Int("cdelay", 0, "% delay chance for packets coming from client (0 - 100)")
	serverDelayChance := flag.Int("sdelay", 0, "% delay chance for packets coming from server (0 - 100)")

	clientDelayMin := flag.Int("cmin", 0, "min delay for packets coming from client in milliseconds")
	clientDelayMax := flag.Int("cmax", 0, "max delay for packets coming from client in milliseconds")
	serverDelayMin := flag.Int("smin", 0, "min delay for packets coming from server in milliseconds")
	serverDelayMax := flag.Int("smax", 0, "max delay for packets coming from server in milliseconds")

	flag.CommandLine.Usage = usage
	flag.Parse()

	if len(flag.Args()) < 4 {
		usage()
		exit(proxyCtx)
	}

	proxyCtx.SIp = flag.Args()[0]
	proxyCtx.SPort = flag.Args()[1]
	proxyCtx.DIp = flag.Args()[2]
	proxyCtx.DPort = flag.Args()[3]

	proxyCtx.ClientDropChance = *clientDropChance
	proxyCtx.ServerDropChance = *serverDropChance

	proxyCtx.ClientDelayChance = *clientDelayChance
	proxyCtx.ServerDelayChance = *serverDelayChance

	proxyCtx.ClientDelayMin = *clientDelayMin
	proxyCtx.ClientDelayMax = *clientDelayMax
	proxyCtx.ServerDelayMin = *serverDelayMin
	proxyCtx.ServerDelayMax = *serverDelayMax

	checkArgs(proxyCtx)

	bind_socket(proxyCtx)
}

func main() {
	proxyCtx := ProxyCtx{}
	parseArgs(&proxyCtx)
}
