package main

import (
	"bytes"
	"encoding/binary"
	"io"
	"log"
	"os"

	"github.com/gotoz/runq/pkg/vm"

	"github.com/pkg/errors"

	"golang.org/x/sys/unix"
)

const (
	defaultBufSize = 4096
	headerSize     = 4 + 1 // 4 byte payload zize + 1 byte message type
)

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
			var writeBuf bytes.Buffer
			readBuf := make([]byte, headerSize)

			_, err := io.ReadAtLeast(fd, readBuf, headerSize)
			if err != nil {
				log.Panic(err)
			}

			payloadSize := int(binary.BigEndian.Uint32(readBuf[:4]))
			msgType := vm.Msgtype(readBuf[4])

			var totalRead int
			for {
				if totalRead == payloadSize {
					break
				}
				if totalRead > payloadSize {
					log.Panicf("totalRead:%d > payloadSize:%d", totalRead, payloadSize)
				}

				bufSize := payloadSize - totalRead
				if bufSize > defaultBufSize {
					bufSize = defaultBufSize
				}

				readBuf = make([]byte, bufSize)
				n, err := fd.Read(readBuf)
				if err != nil {
					log.Panic(err)
				}
				writeBuf.Write(readBuf[:n])
				totalRead += n
			}

			msgChan <- vm.Msg{
				Type: msgType,
				Data: writeBuf.Bytes(),
			}
		}
	}()

	return ackChan, msgChan, nil
}
