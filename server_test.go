package tftp

import (
	"bytes"
	"io"
	"net"
	"os"
	"testing"
	"time"
)

func TestReadServer(t *testing.T) {
	payload1, err := os.ReadFile("./cmd/gopher.png")
	if err != nil {
		t.Fatal(err)
	}

	serverconn, err := net.ListenPacket("udp", "127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = serverconn.Close()
	}()

	tftpServer := TFTPServer{
		Payload:      payload1,
		WriteAllowed: false,
		Timeout:      5 * time.Second,
	}

	go func() {
		_ = tftpServer.Serve(serverconn)
	}()

	rrq := ReadReq{Filename: "gopher.png", Mode: "octet"}

	clientconn, err := net.ListenPacket("udp", "127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = clientconn.Close()
	}()

	rrqbuf, err := rrq.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	n, err := clientconn.WriteTo(rrqbuf, serverconn.LocalAddr())

	if err != nil {
		t.Fatal(err)
	}

	if n != len(rrqbuf) {
		t.Fatalf("expected %d bytes; wrote %d bytes", len(rrqbuf), n)
	}

	// paylaod2 represent the payload that we recieve from the server
	payload2 := new(bytes.Buffer)

	for {
		_ = clientconn.SetReadDeadline(time.Now().Add(time.Second * 5))

		buf := make([]byte, DatagramSize)

		n, addr, err := clientconn.ReadFrom(buf)

		if err != nil {
			t.Fatal(err)
		}

		data := new(Data)

		err = data.UnmarshalBinary(buf[:n])
		if err != nil {
			t.Fatal(err)
		}

		_, err = io.Copy(payload2, data.Payload)
		if err != nil {
			t.Fatal(err)
		}

		// Ack

		ack := Ack{Block: data.Block}

		b, err := ack.MarshalBinary()

		if err != nil {
			t.Fatal(err)
		}

		// Write the acknowledgement packet to the server
		_, err = clientconn.WriteTo(b, addr)
		if err != nil {
			t.Fatal(err)
		}

		// Datagram with less than 516 bytes means its the last packet so break the loop
		if n < DatagramSize {
			break
		}
	}

	if bytes.Equal(payload1, payload2.Bytes()) == false {
		t.Fatal("The sent and recieved payloads are not identical!")
	}
}
