package main

import (
	"flag"
	"fmt"
	"net"
	"time"
	"sync"
	"os"
	"os/signal"
)

var (
	Addr string
	Cmd  string
	Num  uint
	Size uint
	wg   sync.WaitGroup
	quit chan bool
	sc   chan bool // sent packets
	lc   chan bool // lost packets
	ac   chan bool // acctive connections
	pc   chan bool // pending connections
	ec   chan bool // error connections
	pack []byte
)

func init() {
	flag.UintVar(&Num, "n", 10, "number of concurrent clients")
	flag.UintVar(&Size, "s", 1, "packet size (in bytes)")
	flag.Parse()

	Addr = flag.Arg(0)
	if Addr == "" {
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: time %s [options] addr cmd\n", os.Args[0])
	flag.PrintDefaults()
}

func status() {
	wg.Add(1)
	defer wg.Done()
	acs, ecs, pcs, scs, lcs := 0, 0, 0, 0, 0
	fmt.Printf("Benchmarking %s with %d concurrent connections:\n\n", Addr, Num)
	for {
		select {
		case <-ac:
			acs += 1
			pcs -= 1
		case <-ec:
			ecs += 1
			pcs -= 1
		case <-pc:
			pcs += 1
		case <-sc:
			scs += 1
		case <-lc:
			lcs += 1
		case <-quit:
			fmt.Println("")
			return
		}
		fmt.Printf("\rConnections (active: %d, pending: %d, failed: %d), Packets (sent: %d, lost: %d)", acs, pcs, ecs, scs, lcs)
	}
}

func read(conn net.Conn) {
	for {
		buf := make([]byte, 1024)
		conn.Read(buf)
	}
}

func client() {
	conn, err := net.Dial("tcp", Addr)
	if err != nil {
		ec <- true
		return
	}
	ac <- true
	defer conn.Close()
	go read(conn)
	for {
		n, err := conn.Write(pack)
		if err != nil || n != len(pack) {
			lc <- true
		}
		sc <- true
		<-time.After(1 * time.Second)
	}
}

func startAll() {
	for i := 0; i < int(Num); i += 1 {
		pc <- true
		go client()
	}
}

func main() {
	quit = make(chan bool)
	ac = make(chan bool)
	sc = make(chan bool)
	pc = make(chan bool)
	ec = make(chan bool)
	lc = make(chan bool)

	pack = make([]byte, Size)
	for i, _ := range pack {
		pack[i] = 'x'
	}
	
	go status()
	go startAll()

	<-signal.Incoming
	quit <- true
	wg.Wait()
}
