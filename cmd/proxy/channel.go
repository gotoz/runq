package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"

	"github.com/gotoz/runq/pkg/vm"
)

const defaultBufSize = 4096

// MsgChannel is a channel to send messages to init. (write-only)
// AckChannel is a channel to receive messages from init. (read-only)
func mkChannel(sock string) (chan<- vm.Msg, <-chan int, error) {
	msgChan := make(chan vm.Msg, 1)
	ackChan := make(chan int)

	l, err := net.Listen("unix", sock)
	if err != nil {
		return nil, nil, fmt.Errorf("mkChannel net.Listen() failed: %w", err)
	}

	go func() {
		conn, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}

		go func() {
			for {
				msg := <-msgChan
				buf := make([]byte, 4)
				size := uint32(len(msg.Data))
				binary.BigEndian.PutUint32(buf, size)

				buf = append(buf, byte(msg.Type))
				buf = append(buf, msg.Data...)

				if _, err := conn.Write(buf); err != nil {
					log.Printf("conn.Write: %v", err)
				}
			}
		}()

		go func() {
			for {
				buf := make([]byte, defaultBufSize)
				n, err := conn.Read(buf)
				if err == io.EOF {
					return
				}
				if err != nil && err != io.EOF {
					log.Printf("conn.Read: %v", err)
					break
				}
				if n != 1 {
					log.Panicf("ackChan received more than 1 byte: (%d)", n)
				}
				ackChan <- int(buf[0])
			}
		}()
	}()

	return msgChan, ackChan, nil
}
