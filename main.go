package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"sync"
)

var (
	Addr string
	Cmd  string
	Num  uint
	Rep  uint

	wg sync.WaitGroup

	sc chan bool // sent packets
	lc chan bool // lost packets
	fc chan bool // finished connections
	ec chan bool // error connections

	buf []byte
)

func init() {
	flag.UintVar(&Num, "n", 10, "number of concurrent clients")
	flag.UintVar(&Rep, "r", 10, "number of repetitions")
	flag.Parse()

	Addr, Cmd = flag.Arg(0), flag.Arg(1)
	if Addr == "" || Cmd == "" {
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: time %s [options] addr cmd\n", os.Args[0])
	flag.PrintDefaults()
}

func status() {
	fcs, ecs, scs, lcs := 0, 0, 0, 0
	fc = make(chan bool)
	sc = make(chan bool)
	ec = make(chan bool)
	lc = make(chan bool)
	fmt.Printf("Benchmarking %s with %d concurrent clients and %d packets each:\n\n", Addr, Num, Rep)
	for {
		select {
		case <-fc:
			fcs += 1
		case <-ec:
			ecs += 1
		case <-sc:
			scs += 1
		case <-lc:
			lcs += 1
		}
		fmt.Printf("\rClients (done: %d, failed: %d), Packets (sent: %d, lost: %d)", fcs, ecs, scs, lcs)
		if fcs+ecs >= int(Num) {
			break
		}
	}
	fmt.Printf("\n")
	wg.Done()
}

func client() {
	conn, err := net.Dial("tcp", Addr)
	if err != nil {
		ec <- true
		goto done
	}
	for i := 0; i < int(Rep); i += 1 {
		n, err := conn.Write(buf)
		if err != nil || n != len(buf) {
			lc <- true
			continue
		}
		sc <- true
	}
	fc <- true
done:
	wg.Done()
}

func main() {
	go status()
	buf = []byte(Cmd)
	wg.Add(int(Num) + 1)
	for i := 0; i < int(Num); i += 1 {
		go client()
	}
	wg.Wait()
}
