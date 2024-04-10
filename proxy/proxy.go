package main

import (
	"comp7005_project/fsm"
	"context"
	"fmt"
	"net"
	"os"
)

type Key int

const ProxyKey Key = 0

type ProxyCtx struct {
	Socket                 *net.UDPConn
	ClientAddress          *net.UDPAddr
	ServerAddress          *net.UDPAddr
	SIp, DIp, SPort, DPort string
}

func exit(ctx context.Context, t *fsm.Transition) {
	os.Exit(0)
}

func cleanup(ctx context.Context, t *fsm.Transition) {
	proxyCtx, ok := ctx.Value(ProxyKey).(ProxyCtx)
	if !ok {
		t.Fsm.Transition(ctx, "exit")
	}

	if proxyCtx.Socket != nil {
		proxyCtx.Socket.Close()
	}

	t.Fsm.Transition(ctx, "exit")
}

func receive(ctx context.Context, t *fsm.Transition) {
	proxyCtx, ok := ctx.Value(ProxyKey).(ProxyCtx)
	if !ok {
		t.Fsm.Transition(ctx, "cleanup")
	}

	buffer := make([]byte, 1024)
	n, _, err := proxyCtx.Socket.ReadFromUDP(buffer)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("Data:", string(buffer[0:n]))

	t.Fsm.Transition(ctx, "receive")
}

func connectToServer(ctx context.Context, t *fsm.Transition) {
	proxyCtx, ok := ctx.Value(ProxyKey).(ProxyCtx)
	if !ok {
		t.Fsm.Transition(ctx, "exit")
	}

	s, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%s", proxyCtx.DIp, proxyCtx.DPort))
	if err != nil {
		t.Fsm.Transition(ctx, "exit")
	}

	proxyCtx.ClientAddress = s
	_, err = net.DialUDP("udp4", nil, s)
	if err != nil {
		t.Fsm.Transition(context.WithValue(ctx, ProxyKey, proxyCtx), "cleanup")
	}

	fmt.Println("Connected to UDP server at", proxyCtx.ClientAddress)

	t.Fsm.Transition(ctx, "receive")
}

// both
func bind_socket(ctx context.Context, t *fsm.Transition) {
	proxyCtx, ok := ctx.Value(ProxyKey).(ProxyCtx)
	if !ok {
		t.Fsm.Transition(ctx, "exit")
	}

	s, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%s", proxyCtx.SIp, proxyCtx.SPort))
	if err != nil {
		fmt.Println(err)
		t.Fsm.Transition(ctx, "exit")
	}

	connection, err := net.ListenUDP("udp4", s)
	if err != nil {
		fmt.Println(err)
		t.Fsm.Transition(ctx, "exit")
	}

	proxyCtx.ServerAddress = s
	proxyCtx.Socket = connection

	fmt.Println("The UDP server is", proxyCtx.ServerAddress)

	t.Fsm.Transition(context.WithValue(ctx, ProxyKey, proxyCtx), "connect_to_server")
}

func parseArgs(ctx context.Context, t *fsm.Transition) {
	if len(os.Args) < 5 {
		t.Fsm.Transition(ctx, "exit")
	}

	proxyCtx := ProxyCtx{SIp: os.Args[1], SPort: os.Args[2], DIp: os.Args[3], DPort: os.Args[4]}

	t.Fsm.Transition(context.WithValue(ctx, ProxyKey, proxyCtx), "bind_socket")
}

func main() {
	fsm := fsm.Build(
		"start",
		[]fsm.Transitions{
			{Name: "parse_args", From: []string{"start"}, To: "parse_args"},
			{Name: "bind_socket", From: []string{"parse_args"}, To: "bind_socket"},
			{Name: "connect_to_server", From: []string{"bind_socket"}, To: "connect_to_server"},
			{Name: "receive", From: []string{"connect_to_server", "receive"}, To: "receive"},
			{Name: "exit", From: []string{"*"}, To: "exit"},
		},
		[]fsm.Actions{
			{To: "parse_args", Callback: parseArgs},
			{To: "bind_socket", Callback: bind_socket},
			{To: "connect_to_server", Callback: connectToServer},
			{To: "receive", Callback: receive},
			{To: "cleanup", Callback: cleanup},
			{To: "exit", Callback: exit},
		})

	fsm.Transition(context.Background(), "parse_args")
}
