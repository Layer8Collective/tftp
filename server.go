package tftp

import (
	"bytes"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"time"
)

type TFTPServer struct {
	// Whether to accept WriteRequest or not
	WriteAllowed bool
	// Where to save the files
	WriteDir string
	Payload  []byte
	Retries  uint8
	Timeout  time.Duration
}

// Blocking function
func (s TFTPServer) ListenAndServe(addr string) error {
	conn, err := net.ListenPacket("udp", addr)
	if err != nil {
		return err
	}
	defer func() {
		_ = conn.Close()
	}()

	return s.Serve(conn)
}

func (s *TFTPServer) Serve(conn net.PacketConn) error {
	if conn == nil {
		return errors.New("Nil Connection")
	}

	if s.Payload == nil {
		return errors.New("Payload is required")
	}

	if s.Retries == 0 {
		s.Retries = 10
	}

	if s.Timeout == 0 {
		s.Timeout = 4 * time.Second
	}

	var rrq ReadReq
	var wrq WriteReq
	for {
		buf := make([]byte, DatagramSize)
		_, addr, err := conn.ReadFrom(buf)
		if err != nil {
			return err
		}

		err = rrq.UnmarshalBinary(buf)
		if err == nil {
			go s.handleRead(addr.String(), rrq)
			continue
		}

		err = wrq.UnmarshalBinary(buf)
		if err == nil {
			if s.WriteAllowed == false {
				data, _ := Err{Error: ErrIllegalOp, Message: "We don't accept write requests"}.MarshalBinary()
				_, _ = conn.WriteTo(data, addr)
			}
			go s.handleWrite(addr.String(), wrq)
			continue
		}

		log.Printf("[%s] bad request: %v", addr, err)
		continue
	}
}

func (s TFTPServer) handleRead(clientAddr string, rrq ReadReq) {
	log.Printf("[%s] requested read file: %s", clientAddr, rrq.Filename)

	// Using random transfer identifier for each tftp session
	conn, err := net.Dial("udp", clientAddr)
	if err != nil {
		log.Printf("[%s] dial: %v", clientAddr, err)
		return
	}
	defer func() { _ = conn.Close() }()

	var (
		ackPkt  Ack
		errPkt  Err
		dataPkt = Data{Payload: bytes.NewReader(s.Payload)}
		buf     = make([]byte, DatagramSize)
	)

NEXTPACKET:
	for n := DatagramSize; n == DatagramSize; {
		data, err := dataPkt.MarshalBinary()
		if err != nil {
			log.Printf("[%s] preparing data packet: %v", clientAddr, err)
			return
		}
	RETRY:
		for i := s.Retries; i > 0; i-- {
			n, err = conn.Write(data)
			if err != nil {
				log.Printf("[%s] write: %v", clientAddr, err)
				return
			}
			// wait for client's Ack packet
			_ = conn.SetReadDeadline(time.Now().Add(s.Timeout))

			_, err = conn.Read(buf)
			if err != nil {
				if nErr, ok := err.(net.Error); ok && nErr.Timeout() {
					continue RETRY
				}

				log.Printf("[%s] waiting for ACK: %v", clientAddr, err)
				return
			}

			switch {
			case ackPkt.UnmarshalBinary(buf) == nil:
				if uint16(ackPkt.Block) == dataPkt.Block {
					// received ACK; send next data packet
					continue NEXTPACKET
				}

			case errPkt.UnmarshalBinary(buf) == nil:
				log.Printf("[%s] received error: %v",
					clientAddr, errPkt.Message)
				return
			default:
				log.Printf("[%s] bad packet: %v", clientAddr, buf)
			}

		}
		log.Printf("[%s] exhausted retries", clientAddr)
		return
	}
	log.Printf("[%s] send %d blocks", clientAddr, dataPkt.Block)
}

func (s TFTPServer) handleWrite(clientAddr string, wrq WriteReq) {
	log.Printf("[%s] Requested write file: %s", clientAddr, wrq.Filename)

	// Using random transfer identifier for each tftp session
	conn, err := net.Dial("udp", clientAddr)

	if err != nil {
		log.Printf("[%s] dial: %v", clientAddr, err)
		return
	}
	defer conn.Close()

	var (
		ackPkt  Ack
		errPkt  Err
		dataPkt Data
		buf     = make([]byte, DatagramSize)
	)

	// Initial Ack packet to WRQ
	data, err := ackPkt.MarshalBinary()
	if err != nil {
		log.Printf("Can not marshal the ack packet: %s", err)
		return
	}

	_, err = conn.Write(data)

	if err != nil {
		log.Printf("[%s] ack: %v", clientAddr, err)
		return
	}

	file, err := os.Create(wrq.Filename)
	if err != nil {
		log.Printf("[%s] CreateFile: %v", clientAddr, err)
		return
	}

	defer func() {
		err = file.Close()
		if err != nil {
			log.Printf("Can not close the file: %s", err)
		}
	}()

	// Recieve datagrams until the last one comes. last datagram is always less than 516 Bytes.
	for n := DatagramSize; n == DatagramSize; {
		n, err = conn.Read(buf)
		log.Println(n)
		if err != nil {
			log.Printf("Error when reading from connection: %s", err)
			return
		}

		err = errPkt.UnmarshalBinary(buf)
		if err == nil {
			log.Printf("[%s] received error: %v",
				clientAddr, errPkt.Message)
			return
		}

		err = dataPkt.UnmarshalBinary(buf)

		if err != nil {
			log.Println(err)
			return
		}

		data, err := io.ReadAll(dataPkt.Payload)

		if err != nil {
			log.Fatalf("Error reading from reader: %v", err)
			return
		}

		_, err = file.Write(data[:n-4])
		if err != nil {
			log.Printf("can't write the buffer into disk: %s", err)
			return
		}

		ackPkt.Block = dataPkt.Block
		// Acknowledge the data packet
		data, err = ackPkt.MarshalBinary()
		if err != nil {
			log.Printf("Can not marshal the ack packet: %s", err)
			return
		}

		_, err = conn.Write(data)

		if err != nil {
			log.Printf("[%s] ack: %v", clientAddr, err)
			return
		}
	}
	// Out of the loop means we recieved every legit datagram for this connection.
	log.Printf("[%s] Recieved %d blocks of data. Written to the file %s", clientAddr, ackPkt.Block, file.Name())
}
