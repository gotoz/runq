package main

import (
	"flag"
	"log"
	"strings"

	"github.com/pmorjan/kmod"
)

func mainModprobe() {
	a := flag.Bool("a", false, "all")
	r := flag.Bool("r", false, "remove")
	v := flag.Bool("v", false, "verbose")
	va := flag.Bool("va", false, "verbose+all")
	flag.Bool("q", false, "quiet")
	flag.Parse()

	if flag.NArg() < 1 || ((*a || *va) && *r) {
		return
	}

	k, err := kmod.New()
	if err != nil {
		log.Fatal(err)
	}

	args := flag.Args()
	if *r {
		for _, name := range args {
			if err := k.Unload(name); err != nil {
				log.Fatalf("Error: %v", err)
			}
		}
		return
	}
	if *a || *v || *va {
		for _, name := range args {
			if err := k.Load(name, "", 0); err != nil {
				log.Fatalf("Error: %v", err)
			}
		}
		return
	}
	if err := k.Load(args[0], strings.Join(args[1:], " "), 0); err != nil {
		log.Fatalf("Error: %v", err)
	}
}
