package main

import (
	"log"
	"time"

	"github.com/gotoz/runq/internal/cfg"
	"golang.org/x/sys/unix"
)

func reaper() {
	for {
		<-time.After(cfg.ReaperInterval)
		for {
			wpid, err := unix.Wait4(-1, nil, unix.WNOHANG, nil)
			if err != nil {
				log.Printf("Wait failed: %w", err)
				break
			}
			if wpid <= 0 { //  -1 Error, 0 no childs
				break
			}
		}
	}
}
