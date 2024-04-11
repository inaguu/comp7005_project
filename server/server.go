package main

import (
	"bytes"
	"comp7005_project/fsm"
	"context"
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

	rand.Seed(time.Now().Unix())

	data := []byte(strconv.Itoa(random(1, 1001)))
	fmt.Printf("\ndata: %s\n", string(data))

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
	if err != nil {
		fmt.Println(err)
		t.Fsm.Transition(ctx, "cleanup")
	}

	fmt.Print("-> ", string(buffer[0:n]))

	serverCtx.ClientAddress = addr

	t.Fsm.Transition(context.WithValue(ctx, ServerKey, serverCtx), "send")
}

func synReceive(ctx context.Context, t *fsm.Transition) {
	serverCtx, ok := ctx.Value(ServerKey).(ServerCtx)
	if !ok {
		t.Fsm.Transition(ctx, "cleanup")
	}

	buffer := make([]byte, 1024)

	n, _, err := serverCtx.Socket.ReadFromUDP(buffer)
	if err != nil {
		fmt.Println(err)
		t.Fsm.Transition(ctx, "cleanup")
	}

	buf := bytes.NewBuffer(buffer[0:n])
	var synPacket packet

	errPacket := binary.Read(buf, binary.BigEndian, &synPacket)
	if errPacket != nil {
		fmt.Println("failed to Read:", errPacket)
		return
	}

	fmt.Printf("-> %v\n", synPacket)

	t.Fsm.Transition(context.WithValue(ctx, ServerKey, serverCtx), "wait_for_ack")
}

func waitForAck(ctx context.Context, t *fsm.Transition) {
	t.Fsm.Transition(ctx, "receive")
}

func decryptPacket(ctx context.Context) {

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

	connection, err := net.ListenUDP("udp4", s)
	if err != nil {
		fmt.Println(err)
		t.Fsm.Transition(ctx, "exit")
	}

	serverCtx.Socket = connection

	t.Fsm.Transition(context.WithValue(ctx, ServerKey, serverCtx), "syn_recv")
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
			{Name: "syn_recv", From: []string{"bind_socket"}, To: "syn_recv"},
			{Name: "wait_for_ack", From: []string{"syn_recv"}, To: "wait_for_ack"},
			{Name: "receive", From: []string{"wait_for_ack", "send"}, To: "receive"},
			{Name: "send", From: []string{"receive"}, To: "send"},
			{Name: "cleanup", From: []string{"bind_socket", "receive", "send"}, To: "cleanup"},
			{Name: "exit", From: []string{"*"}, To: "end"},
		},
		[]fsm.Actions{
			{To: "parse_args", Callback: parseArgs},
			{To: "bind_socket", Callback: bindSocket},
			{To: "syn_recv", Callback: synReceive},     // send a syn/ack upon receving an ack from a client
			{To: "wait_for_ack", Callback: waitForAck}, // gets the ack from a client
			{To: "receive", Callback: receive},
			{To: "send", Callback: send},
			{To: "cleanup", Callback: cleanup},
			{To: "exit", Callback: exit},
		})

	fsm.Transition(context.Background(), "parse_args")
}
