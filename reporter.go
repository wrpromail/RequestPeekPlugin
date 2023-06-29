package RequestPeekPlugin

import (
	"log"
	"net"
)

type reporter interface {
	Report(data []byte)
}

type udpReporter struct {
	conn net.Conn
	addr string
}

func newUdpReporter(addr string) *udpReporter {
	conn, err := net.Dial("udp", addr)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	return &udpReporter{conn: conn, addr: addr}
}

func (r *udpReporter) Report(data []byte) {
	r.conn.Write(data)
}
