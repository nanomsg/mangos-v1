package main

import (
	"flag"
	"log"
	"time"

	"github.com/go-mangos/mangos"
	"github.com/go-mangos/mangos/protocol/rep"
	"github.com/go-mangos/mangos/transport/tcp"
)

func main() {
	var url = flag.String("url", "tcp://127.0.0.1:40899", "REP socket to listen on")
	var recDl = flag.Duration("recv", 2*time.Second, "receive timeout (deadline until next Recv times out)")
	var sndDl = flag.Duration("send", 1*time.Second, "send timeout (deadline until Send times out)")
	var reCon = flag.Duration("recon", 100*time.Millisecond, "connection retry interval")
	var reConMax = flag.Duration("reconMax", 2*time.Second, "maximum amount of time between connection attempts")
	var wql = flag.Int("wq", 1, "length (in #messages) of the write channel attached to the socket")
	var rql = flag.Int("rq", 1, "length (in #messages) of the read channel attached to the socket")
	var crash = flag.Bool("crash", false, "crash after receiving the first request")
	var sleep = flag.Duration("sleep", 0, "time between receiving REQ and sending REP - to simulate 'work'")

	flag.Parse()

	// open REP side of socket
	sock, err := rep.NewSocket()
	if err != nil {
		log.Fatalf("failed to open REP socket: %s", err)
	}
	defer sock.Close()

	proto := sock.GetProtocol()
	log.Printf("opened socket with protocol %q (%d)", proto.Name(), proto.Number())

	// Set the receive timeout for the reply - deals with the missing-Ack case
	if err = sock.SetOption(mangos.OptionRecvDeadline, *recDl); err != nil {
		log.Fatalf("failed to set receive deadline to %s: %s", *recDl, err)
	} else if err = sock.SetOption(mangos.OptionSendDeadline, *sndDl); err != nil {
		log.Fatalf("failed to set send deadline to %s: %s", *sndDl, err)
	} else if err = sock.SetOption(mangos.OptionWriteQLen, *wql); err != nil {
		log.Fatalf("failed to set the write queue length to %d: %s", *wql)
	} else if err = sock.SetOption(mangos.OptionReadQLen, *rql); err != nil {
		log.Fatalf("failed to set the read queue length to %d: %s", *rql)
	} else if err = sock.SetOption(mangos.OptionReconnectTime, *reCon); err != nil {
		log.Fatalf("failed to set the reconnect time to %s: %s", *reCon, err)
	} else if err = sock.SetOption(mangos.OptionMaxReconnectTime, *reConMax); err != nil {
		log.Fatalf("failed to set the maximum reconnect time to %s: %s", *reConMax, err)
	}
	log.Printf("sockopts: %s", SockOpts(sock))

	sock.AddTransport(tcp.NewTransport())
	sock.SetPortHook(portHook)

	if err := sock.Listen(*url); err != nil {
		log.Fatalf("failed to listen on %s: %s", *url, err)
	}

	for {
		log.Printf("waiting for request ...")

		msg, err := sock.RecvMsg()
		if err != nil {
			log.Fatalf("failed to receive message on %s: %s", *url, err)
		}

		log.Printf("server: received request: %q, port: %s", string(msg.Body), Port2Str(msg.Port))
		if *crash {
			log.Fatalf("server: simulating a crash")
		}
		if *sleep > 0 {
			log.Printf("server: sleeping %s ...", *sleep)
			time.Sleep(*sleep)
		}

		log.Printf("sending reply ...")
		if err := sock.Send([]byte("World")); err != nil {
			log.Fatalf("failed to send 'World' reply: %s", err)
		}
	}
}
