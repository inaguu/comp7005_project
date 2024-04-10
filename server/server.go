package main

import (
	"comp7005_project/fsm"
	"context"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type Key int

type ServerCtx struct {
	Socket        *net.UDPConn
	ClientAddress *net.UDPAddr
	Ip, Port      string
}

const (
	INPUT_ERROR     = "Usage: <filename> -i <ip address> -p <port_number>"
	ServerKey   Key = 0
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

func exit(ctx context.Context, t *fsm.Transition) {
	fmt.Println("Exiting...")
	os.Exit(0)
}

func cleanup(ctx context.Context, t *fsm.Transition) {
	serverCtx, ok := ctx.Value(ServerKey).(ServerCtx)
	if !ok {
		t.Fsm.Transition(ctx, "exit")
	}

	if serverCtx.Socket != nil {
		serverCtx.Socket.Close()
	}
}

func send(ctx context.Context, t *fsm.Transition) {
	serverCtx, ok := ctx.Value(ServerKey).(ServerCtx)
	if !ok {
		t.Fsm.Transition(ctx, "cleanup")
	}
	fmt.Println(serverCtx.Socket, serverCtx.ClientAddress)

	rand.Seed(time.Now().Unix())

	data := []byte(strconv.Itoa(random(1, 1001)))
	fmt.Printf("data: %s\n", string(data))

	_, err := serverCtx.Socket.WriteToUDP(data, serverCtx.ClientAddress)
	if err != nil {
		fmt.Println(err)
		t.Fsm.Transition(ctx, "cleanup")
	}

	t.Fsm.Transition(ctx, "receive")
}

func receive(ctx context.Context, t *fsm.Transition) {
	serverCtx, ok := ctx.Value(ServerKey).(ServerCtx)
	if !ok {
		t.Fsm.Transition(ctx, "cleanup")
	}

	buffer := make([]byte, 1024)

	n, addr, err := serverCtx.Socket.ReadFromUDP(buffer)

	fmt.Print("-> ", string(buffer[0:n-1]))

	if strings.TrimSpace(string(buffer[0:n])) == "STOP" || err != nil {
		fmt.Println("Exiting UDP server!")
		t.Fsm.Transition(ctx, "cleanup")
	}
	serverCtx.ClientAddress = addr

	t.Fsm.Transition(context.WithValue(ctx, ServerKey, serverCtx), "send")
}

func bindSocket(ctx context.Context, t *fsm.Transition) {
	serverCtx, ok := ctx.Value(ServerKey).(ServerCtx)
	if !ok {
		t.Fsm.Transition(ctx, "cleanup")
	}

	s, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%s", serverCtx.Ip, serverCtx.Port))
	if err != nil {
		fmt.Println(err)
		t.Fsm.Transition(ctx, "exit")
	}
	fmt.Println(s)

	connection, err := net.ListenUDP("udp4", s)
	if err != nil {
		fmt.Println(err)
		t.Fsm.Transition(ctx, "exit")
	}
	fmt.Println(connection)

	serverCtx.Socket = connection

	t.Fsm.Transition(context.WithValue(ctx, ServerKey, serverCtx), "receive")
}

func parseArgs(ctx context.Context, t *fsm.Transition) {
	if len(os.Args) < 3 {
		fmt.Println(INPUT_ERROR)
		t.Fsm.Transition(ctx, "exit")
	}

	serverCtx := ServerCtx{Ip: os.Args[1], Port: os.Args[2]}

	t.Fsm.Transition(context.WithValue(ctx, ServerKey, serverCtx), "bind_socket")
}

func main() {
	fsm := fsm.Build(
		"start",
		[]fsm.Transitions{
			{Name: "parse_args", From: []string{"start"}, To: "parse_args"},
			{Name: "bind_socket", From: []string{"parse_args"}, To: "bind_socket"},
			{Name: "receive", From: []string{"bind_socket", "send"}, To: "receive"},
			{Name: "send", From: []string{"receive"}, To: "send"},
			{Name: "cleanup", From: []string{"bind_socket", "receive", "send"}, To: "cleanup"},
			{Name: "exit", From: []string{"*"}, To: "end"},
		},
		[]fsm.Actions{
			{To: "parse_args", Callback: parseArgs},
			{To: "bind_socket", Callback: bindSocket},
			{To: "receive", Callback: receive},
			{To: "send", Callback: send},
			{To: "cleanup", Callback: cleanup},
			{To: "exit", Callback: exit},
		})

	fsm.Transition(context.Background(), "parse_args")
}
