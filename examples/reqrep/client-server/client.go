package main

import (
	"encoding/binary"
	"flag"
	"log"
	"time"

	"github.com/go-mangos/mangos"
	"github.com/go-mangos/mangos/protocol/req"
	"github.com/go-mangos/mangos/transport/tcp"
)

func main() {
	var url = flag.String("url", "tcp://127.0.0.1:40899", "REP socket to connect to")
	var recDl = flag.Duration("recv", 2*time.Second, "receive timeout (deadline until next Recv times out)")
	var sndDl = flag.Duration("send", 1*time.Second, "send timeout (deadline until Send times out)")
	var retry = flag.Duration("retry", 5*time.Second, "send retry interval in between sends")
	var reCon = flag.Duration("recon", 100*time.Millisecond, "connection retry interval")
	var reConMax = flag.Duration("reconMax", 2*time.Second, "maximum amount of time between connection attempts")
	// A wql > 0 causes the send timeout to become meaningless, since all (retried) messages silently expire.
	var wql = flag.Int("wq", 0, "length (in #messages) of the write channel attached to the socket")
	var rql = flag.Int("rq", 1, "length (in #messages) of the read channel attached to the socket")

	flag.Parse()

	// initialize new REQ socket
	sock, err := req.NewSocket()
	if err != nil {
		log.Fatalf("failed to set up REQ socket: %s", err)
	}
	defer sock.Close()

	proto := sock.GetProtocol()
	log.Printf("opened socket with protocol %q (%d)", proto.Name(), proto.Number())

	// Socket options
	if err = sock.SetOption(mangos.OptionRecvDeadline, *recDl); err != nil {
		log.Fatalf("failed to set receive deadline to %s: %s", *recDl, err)
	} else if err = sock.SetOption(mangos.OptionSendDeadline, *sndDl); err != nil {
		log.Fatalf("failed to set send deadline to %s: %s", *sndDl, err)
	} else if err = sock.SetOption(mangos.OptionRetryTime, *retry); err != nil {
		log.Fatalf("failed to set retry interal to %s: %s", *retry, err)
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

	// tcp Transport implements the Transport interface (Scheme, NewDialer, NewListener)
	sock.AddTransport(tcp.NewTransport())

	sock.SetPortHook(portHook)

	if err := sock.Dial(*url); err != nil {
		log.Fatalf("failed to dial %s: %s", *url, err)
	}

	log.Printf("client queueing 'Hello' message")
	if err := sock.Send([]byte("Hello")); err != nil {
		log.Fatalf("failed to send 'Hello' message: %s", err)
	}

	//log.Fatalf("client simulating a crash!")
	reply, err := sock.RecvMsg()
	if err != nil {
		log.Fatalf("failed to receive from %s: %s", *url, err)
	}

	id := binary.BigEndian.Uint32(reply.Header)
	log.Printf("reply: %q, ID #%d, port %s", string(reply.Body), id, Port2Str(reply.Port))
}
