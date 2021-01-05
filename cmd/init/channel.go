package main

import (
	"bufio"
	"encoding/binary"
	"io"
	"log"
	"os"

	"github.com/gotoz/runq/pkg/vm"

	"github.com/pkg/errors"

	"golang.org/x/sys/unix"
)

const headerSize = 4 + 1 // 4 byte payload size + 1 byte message type

// AckChannel to send acknowledge messages to proxy.
// MsgChannel to receive messages from proxy.
func mkChannel(path string) (chan uint8, <-chan vm.Msg, error) {
	fd, err := os.OpenFile(path, unix.O_RDWR, 0600|os.ModeExclusive)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "OpenFile %s", path)
	}

	ackChan := make(chan uint8)
	msgChan := make(chan vm.Msg, 1)

	go func() {
		for {
			value := <-ackChan
			if _, err := fd.Write([]byte{value}); err != nil {
				log.Printf("Write: %v", err)
			}
			ackChan <- 0
		}
	}()

	go func() {
		for {
			// read header
			buf := make([]byte, headerSize)
			if _, err := io.ReadFull(fd, buf); err != nil {
				log.Panic(err)
			}

			payloadSize := int(binary.BigEndian.Uint32(buf[:4]))
			msgType := vm.Msgtype(buf[4])

			// read payload
			buf = make([]byte, payloadSize)
			rd := bufio.NewReaderSize(fd, payloadSize)
			if _, err := io.ReadFull(rd, buf); err != nil {
				log.Panic(err)
			}

			msgChan <- vm.Msg{
				Type: msgType,
				Data: buf,
			}
		}
	}()

	return ackChan, msgChan, nil
}
