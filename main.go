package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

var (
	Addr string
	Num  uint
	Size uint
	quit chan struct{}
	sc   chan struct{} // sent packets
	lc   chan struct{} // lost packets
	ac   chan struct{} // acctive connections
	pc   chan struct{} // pending connections
	ec   chan struct{} // error connections
	pack []byte
)

func init() {
	flag.UintVar(&Num, "n", 10, "number of concurrent connections")
	flag.UintVar(&Size, "s", 1, "packet size (in bytes)")
	flag.Parse()

	Addr = flag.Arg(0)
	if Addr == "" {
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [options] addr cmd\n", os.Args[0])
	flag.PrintDefaults()
}

func status(wg *sync.WaitGroup) {
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
		select {
		case <-quit:
			return
		default:
			buf := make([]byte, 1024)
			_, _ = conn.Read(buf)
		}
	}
}

func client() {
	conn, err := net.Dial("tcp", Addr)
	if err != nil {
		ec <- struct{}{}
		return
	}
	defer conn.Close()

	ac <- struct{}{}
	go read(conn)
	for {
		select {
		case <-quit:
			return
		default:
			n, err := conn.Write(pack)
			if err != nil || n != len(pack) {
				lc <- struct{}{}
			}
			sc <- struct{}{}
			<-time.After(1 * time.Second)
		}
	}
}

func startAll() {
	for i := 0; i < int(Num); i += 1 {
		pc <- struct{}{}
		go client()
	}
}

func main() {
	quit = make(chan struct{}, 2*Num+1)
	ac = make(chan struct{}, 128)
	sc = make(chan struct{}, 128)
	pc = make(chan struct{}, 128)
	ec = make(chan struct{}, 128)
	lc = make(chan struct{}, 128)

	pack = make([]byte, Size)
	for i := range pack {
		pack[i] = 'x'
	}

	wg := &sync.WaitGroup{}
	defer func() {
		wg.Wait()
	}()

	wg.Add(1)
	go status(wg)
	startAll()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	<-sig

	var i uint
	for i = 0; i < 2*Num+1; i++ {
		quit <- struct{}{}
	}
}
