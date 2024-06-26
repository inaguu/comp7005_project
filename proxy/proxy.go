package main

import (
	"comp7005_project/utils"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"os"
	"time"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"
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

	ClientPackets []utils.PacketAndTime
	ServerPackets []utils.PacketAndTime
	initialPacket bool
	initialTime   time.Time
}

func group(packets []utils.PacketAndTime) [][]float64 {
	packetMap := make(map[float64]int)
	groups := make([][]float64, 0)
	for _, packet := range packets {
		packetMap[packet.Time]++
	}

	for k, v := range packetMap {
		groups = append(groups, []float64{k, float64(v)})
	}

	return groups
}

func duplicates(packets []utils.PacketAndTime) []utils.PacketAndTime {
	var dupes []utils.PacketAndTime
	for _, packet := range packets {
		if packet.Packet.Header.Flags.DUP {
			dupes = append(dupes, packet)
		}
	}

	return dupes
}

func generateGraph(proxyCtx *ProxyCtx) {
	plotPoints := func(points [][]float64) plotter.XYs {
		pts := make(plotter.XYs, len(points))
		for i := range pts {
			pts[i].X = float64(points[i][0])
			pts[i].Y = float64(points[i][1])
		}

		return pts
	}

	var clientRetransmissions [][]float64
	var serverRetransmissions [][]float64

	for {
		p := plot.New()
		p.Title.Text = "Client and Server Retransmissions"
		p.X.Label.Text = "Time (seconds)"
		p.Y.Label.Text = "Retransmissions"
		p.Y.Min = 0

		newPoints := group(duplicates(proxyCtx.ClientPackets))
		newPointsS := group(duplicates(proxyCtx.ServerPackets))
		if len(newPoints) == 0 {
			clientRetransmissions = append(clientRetransmissions, []float64{time.Since(proxyCtx.initialTime).Seconds(), 0})
		} else {
			clientRetransmissions = append(clientRetransmissions, newPoints...)
		}

		if len(newPointsS) == 0 {
			serverRetransmissions = append(serverRetransmissions, []float64{time.Since(proxyCtx.initialTime).Seconds(), 0})
		} else {
			serverRetransmissions = append(serverRetransmissions, newPointsS...)
		}

		err := plotutil.AddLinePoints(p, "Client", plotPoints(clientRetransmissions), "Server", plotPoints(serverRetransmissions))
		if err != nil {
			panic(err)
		}
		if err := p.Save(8*vg.Inch, 4*vg.Inch, "retransmissions.png"); err != nil {
			panic(err)
		}

		time.Sleep(1 * time.Second)
	}

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

func dropPacket(dropChance int) bool {
	chance := rand.Intn(100)
	return chance < dropChance
}

func delayPacket(delayChance int) bool {
	chance := rand.Intn(100)
	return chance < delayChance
}

func randRange(min int, max int) int {
	return rand.Intn(max+1-min) + min
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

	if sendTo(addr.String(), proxyCtx.ServerAddress.String()) {
		proxyCtx.ServerPackets = append(proxyCtx.ServerPackets, utils.PacketAndTime{Time: float64(time.Since(proxyCtx.initialTime).Seconds()), Packet: packet})

		dropChance := rand.Intn(100)

		if dropChance < proxyCtx.ServerDropChance {
			fmt.Println("Packet dropped from server:", packetString(packet))
			receive(proxyCtx)
		}

		delayChance := rand.Intn(100)
		if delayChance < proxyCtx.ServerDelayChance {
			delayTime := randRange(proxyCtx.ServerDelayMin, proxyCtx.ServerDelayMax)
			fmt.Printf("Packet Delayed from server for %d ms: %s\n", delayTime, packetString(packet))
			go func() {
				time.Sleep(time.Duration(delayTime) * time.Millisecond)
				sendToClient(proxyCtx)
			}()
			receive(proxyCtx)
		} else {
			sendToClient(proxyCtx)
		}

	} else {
		proxyCtx.ClientPackets = append(proxyCtx.ClientPackets, utils.PacketAndTime{Time: time.Since(proxyCtx.initialTime).Seconds(), Packet: packet})

		dropChance := rand.Intn(100)

		if dropChance < proxyCtx.ClientDropChance {
			fmt.Println("Packet Dropped from client:", packetString(packet))
			receive(proxyCtx)
		}

		delayChance := rand.Intn(100)
		if delayChance < proxyCtx.ClientDelayChance {
			delayTime := randRange(proxyCtx.ClientDelayMin, proxyCtx.ClientDelayMax)
			fmt.Printf("Packet Delayed from client for %d ms: %s\n", delayTime, packetString(packet))
			go func() {
				time.Sleep(time.Duration(delayTime) * time.Millisecond)
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
	s, err := net.ResolveUDPAddr("udp", utils.Address(proxyCtx.DIp, proxyCtx.DPort))
	if err != nil {
		fmt.Println(err)
		exit(proxyCtx)
	}

	proxyCtx.ServerAddress = s
	_, err = net.DialUDP("udp", nil, s)
	if err != nil {
		cleanup(proxyCtx)
	}

	fmt.Println("Connected to UDP server at", proxyCtx.ServerAddress)

	receive(proxyCtx)
}

// both
func bindSocket(proxyCtx *ProxyCtx) {
	s, err := net.ResolveUDPAddr("udp", utils.Address(proxyCtx.SIp, proxyCtx.SPort))
	if err != nil {
		fmt.Println(err)
		exit(proxyCtx)
	}

	connection, err := net.ListenUDP("udp", s)
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

func checkAddresses(proxyCtx *ProxyCtx) {
	errorString := ""

	proxyAddress := utils.Address(proxyCtx.SIp, proxyCtx.SPort)
	serverAddress := utils.Address(proxyCtx.DIp, proxyCtx.DPort)

	if proxyAddress == "" {
		errorString = fmt.Sprintf("%s and %s for proxy is not a valid ip and port combination", proxyCtx.SIp, proxyCtx.SPort)
	} else if serverAddress == "" {
		errorString = fmt.Sprintf("%s and %s for server is not a valid ip and port combination", proxyCtx.SIp, proxyCtx.SPort)
	}

	if errorString != "" {
		fmt.Fprintf(flag.CommandLine.Output(), "%s\n", errorString)
		usage()
		exit(proxyCtx)
	}
}

func checkFlags(proxyCtx *ProxyCtx) {
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

func checkArgs(proxyCtx *ProxyCtx) {
	checkFlags(proxyCtx)
	checkAddresses(proxyCtx)
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
		fmt.Fprintln(flag.CommandLine.Output(), "not enough arguments")
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
	bindSocket(proxyCtx)
}

func main() {
	proxyCtx := ProxyCtx{}
	proxyCtx.initialPacket = true
	proxyCtx.initialTime = time.Now()
	go generateGraph(&proxyCtx)
	parseArgs(&proxyCtx)
}
