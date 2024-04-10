package main

import (
	"comp7005_project/fsm"
	"context"
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

	t.Fsm.Transition(ctx, "cleanup")
}

func send(ctx context.Context, t *fsm.Transition) {
	clientCtx, ok := ctx.Value(ClientKey).(ClientCtx)
	if !ok {
		t.Fsm.Transition(ctx, "cleanup")
	}

	_, err := clientCtx.Socket.Write([]byte(clientCtx.Data))
	if err != nil {
		fmt.Println(err)
		t.Fsm.Transition(ctx, "cleanup")
	}
	fmt.Println("sent", clientCtx.Data)
	t.Fsm.Transition(ctx, "receive")
}

func waitForSynAck(ctx context.Context, t *fsm.Transition) {
	t.Fsm.Transition(ctx, "send")
}

func sendSyn(ctx context.Context, t *fsm.Transition) {
	t.Fsm.Transition(ctx, "wait_for_syn_ack")
}

func readFile(ctx context.Context, t *fsm.Transition) {
	clientCtx, ok := ctx.Value(ClientKey).(ClientCtx)
	if !ok {
		t.Fsm.Transition(ctx, "exit")
	}

	content, err := os.ReadFile(clientCtx.FilePath)
	if err != nil {
		fmt.Println("Read File Error:\n", err)
	}

	if len(content) == 0 {
		fmt.Println("File is empty")
	}

	clientCtx.Data = string(content)
	t.Fsm.Transition(context.WithValue(ctx, ClientKey, clientCtx), "send_syn")
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
	t.Fsm.Transition(context.WithValue(ctx, ClientKey, clientCtx), "read_file")
}

func parseArgs(ctx context.Context, t *fsm.Transition) {
	arguments := os.Args
	if len(arguments) == 1 {
		fmt.Println("Please provide a host:port string")
		t.Fsm.Transition(ctx, "exit")
	}
	clientCtx := ClientCtx{Address: os.Args[1], FilePath: os.Args[2]}

	t.Fsm.Transition(context.WithValue(ctx, ClientKey, clientCtx), "bind_socket")
}

func main() {
	fsm := fsm.Build(
		"start",
		[]fsm.Transitions{
			{Name: "parse_args", From: []string{"start"}, To: "parse_args"},
			{Name: "bind_socket", From: []string{"parse_args"}, To: "bind_socket"},
			{Name: "read_file", From: []string{"bind_socket"}, To: "read_file"},
			{Name: "send_syn", From: []string{"read_file"}, To: "send_syn"},
			{Name: "wait_for_syn_ack", From: []string{"send_syn"}, To: "wait_for_syn_ack"},
			{Name: "send", From: []string{"wait_for_syn_ack"}, To: "send"},
			{Name: "receive", From: []string{"send"}, To: "receive"},
			{Name: "cleanup", From: []string{"bind_socket", "user_input", "send", "receive"}, To: "cleanup"},
			{Name: "exit", From: []string{"*"}, To: "end"},
		},
		[]fsm.Actions{
			{To: "parse_args", Callback: parseArgs},
			{To: "bind_socket", Callback: bindSocket},
			{To: "read_file", Callback: readFile},
			{To: "send_syn", Callback: sendSyn},
			{To: "wait_for_syn_ack", Callback: waitForSynAck},
			{To: "send", Callback: send},
			{To: "receive", Callback: receive},
			{To: "cleanup", Callback: cleanup},
			{To: "exit", Callback: exit},
		})

	fsm.Transition(context.Background(), "parse_args")
}
