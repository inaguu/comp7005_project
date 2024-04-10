package main

import (
	"bufio"
	"comp7005_project/fsm"
	"context"
	"fmt"
	"net"
	"os"
	"strings"
)

type Key int

const ClientKey Key = 0

type ClientCtx struct {
	Socket        *net.UDPConn
	Address, Text string
}

func exit(ctx context.Context, t *fsm.Transition) {
	fmt.Println("Exiting...")
	os.Exit(0)
}

func cleanup(ctx context.Context, t *fsm.Transition) {
	clientCtx, ok := ctx.Value(ClientKey).(ClientCtx)
	if !ok {
		t.Fsm.Transition(ctx, "exit")
	}

	if clientCtx.Socket != nil {
		clientCtx.Socket.Close()
	}

	t.Fsm.Transition(ctx, "exit")
}

func receive(ctx context.Context, t *fsm.Transition) {
	clientCtx, ok := ctx.Value(ClientKey).(ClientCtx)
	if !ok {
		t.Fsm.Transition(ctx, "cleanup")
	}

	buffer := make([]byte, 1024)

	n, _, err := clientCtx.Socket.ReadFromUDP(buffer)
	if err != nil {
		fmt.Println(err)
		t.Fsm.Transition(ctx, "cleanup")
	}

	fmt.Printf("Reply: %s\n", string(buffer[0:n]))

	t.Fsm.Transition(ctx, "user_input")
}

func send(ctx context.Context, t *fsm.Transition) {
	clientCtx, ok := ctx.Value(ClientKey).(ClientCtx)
	if !ok {
		t.Fsm.Transition(ctx, "cleanup")
	}

	data := []byte(clientCtx.Text + "\n")

	_, err := clientCtx.Socket.Write(data)

	if strings.TrimSpace(string(data)) == "STOP" {
		fmt.Println("Exiting UDP client!")
		t.Fsm.Transition(ctx, "cleanup")
	}

	if err != nil {
		fmt.Println(err)
		t.Fsm.Transition(ctx, "cleanup")
	}

	t.Fsm.Transition(ctx, "receive")
}

func userInput(ctx context.Context, t *fsm.Transition) {
	clientCtx, ok := ctx.Value(ClientKey).(ClientCtx)
	if !ok {
		t.Fsm.Transition(ctx, "cleanup")
	}

	reader := bufio.NewReader(os.Stdin)

	fmt.Print(">> ")
	text, _ := reader.ReadString('\n')
	clientCtx.Text = text

	t.Fsm.Transition(context.WithValue(ctx, ClientKey, clientCtx), "send")
}

func bindSocket(ctx context.Context, t *fsm.Transition) {
	clientCtx, ok := ctx.Value(ClientKey).(ClientCtx)
	if !ok {
		t.Fsm.Transition(ctx, "exit")
	}

	s, _ := net.ResolveUDPAddr("udp4", clientCtx.Address)
	c, err := net.DialUDP("udp4", nil, s)
	if err != nil {
		fmt.Println(err)
		t.Fsm.Transition(ctx, "exit")
	}

	clientCtx.Socket = c

	fmt.Printf("The UDP server is %s\n", clientCtx.Socket.RemoteAddr().String())
	t.Fsm.Transition(context.WithValue(ctx, ClientKey, clientCtx), "user_input")
}

func parseArgs(ctx context.Context, t *fsm.Transition) {
	arguments := os.Args
	if len(arguments) == 1 {
		fmt.Println("Please provide a host:port string")
		t.Fsm.Transition(ctx, "exit")
	}
	clientCtx := ClientCtx{Address: os.Args[1]}

	t.Fsm.Transition(context.WithValue(ctx, ClientKey, clientCtx), "bind_socket")
}

func main() {
	fsm := fsm.Build(
		"start",
		[]fsm.Transitions{
			{Name: "parse_args", From: []string{"start"}, To: "parse_args"},
			{Name: "bind_socket", From: []string{"parse_args"}, To: "bind_socket"},
			{Name: "user_input", From: []string{"bind_socket", "receive"}, To: "user_input"},
			{Name: "send", From: []string{"user_input"}, To: "send"},
			{Name: "receive", From: []string{"send"}, To: "receive"},
			{Name: "cleanup", From: []string{"bind_socket", "user_input", "send", "receive"}, To: "cleanup"},
			{Name: "exit", From: []string{"*"}, To: "end"},
		},
		[]fsm.Actions{
			{To: "parse_args", Callback: parseArgs},
			{To: "bind_socket", Callback: bindSocket},
			{To: "user_input", Callback: userInput},
			{To: "send", Callback: send},
			{To: "receive", Callback: receive},
			{To: "cleanup", Callback: cleanup},
			{To: "exit", Callback: exit},
		})

	fsm.Transition(context.Background(), "parse_args")
}
