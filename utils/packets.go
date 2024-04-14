package utils

import (
	"bytes"
	"encoding/gob"
	"net"
)

type Flags struct {
	SYN, FIN, ACK, PSH, RST, URG, CWD, ECE bool
}

type Header struct {
	Flags         Flags
	Seq, Ack, Len uint32
	Win           uint16
}

type Packet struct {
	SrcAddr, DstAddr, Data string
	Header                 Header
}

func EncodePacket(header Packet) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := gob.NewEncoder(&buffer)

	if err := encoder.Encode(header); err != nil {
		return make([]byte, 0), err
	}

	return buffer.Bytes(), nil
}

func DecodePacket(encoded []byte) (Packet, error) {
	var packet Packet
	decoder := gob.NewDecoder(bytes.NewBuffer(encoded))

	if err := decoder.Decode(&packet); err != nil {
		return Packet{}, err
	}

	return packet, nil
}

func Address(ip string, port string) string {
	unparsedIp := ip
	if ip == "localhost" {
		unparsedIp = "127.0.0.1"
	}

	parsedIp := net.ParseIP(unparsedIp)

	if parsedIp == nil {
		return ""
	}

	return net.JoinHostPort(parsedIp.String(), port)
}
