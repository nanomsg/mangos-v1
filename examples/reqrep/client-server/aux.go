package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-mangos/mangos"
)

func Port2Str(port mangos.Port) string {
	local, err := port.GetProp(mangos.PropLocalAddr)
	if err != nil {
		log.Fatalf("failed to obtain local port address: %s", err)
	}
	remote, err := port.GetProp(mangos.PropRemoteAddr)
	if err != nil {
		log.Fatalf("failed to obtain remote port address: %s", err)
	}

	epType := "server"
	if !port.IsServer() {
		epType = "client"
	}
	epState := "open"
	if !port.IsOpen() {
		epState = "closed"
	}
	id := "??"
	if ep, ok := port.(mangos.Endpoint); ok {
		id = fmt.Sprintf("%d", ep.GetID())
	}

	return fmt.Sprintf("%s, local: %s %s~%d, remote: %s~%d#%s",
		epState, epType, local, port.LocalProtocol(),
		remote, port.RemoteProtocol(), id)
}

// simple port hook that just prints when a port/endpoint is added/removed
var slept bool

func portHook(action mangos.PortAction, port mangos.Port) bool {
	if action == mangos.PortActionAdd {
		log.Printf("porthook - adding port: %s", Port2Str(port))
		if os.Getenv("PORTSLEEP") != "" && !slept {
			var portsleep = 10 * time.Second
			if val, err := time.ParseDuration(os.Getenv("PORTSLEEP")); err == nil {
				portsleep = val
			}
			log.Printf("porthook: sleeping %s to allow for connection drop", portsleep)
			time.Sleep(portsleep)
			slept = true
		}
	} else if action == mangos.PortActionRemove {
		log.Printf("porthook - removing port: %s", Port2Str(port))
	}
	return true
}

// SockOpts pretty-prints socket options
func SockOpts(sock mangos.Socket) string {
	rdl, err := sock.GetOption(mangos.OptionRecvDeadline)
	if err != nil {
		log.Fatalf("failed to get receive deadline value: %s", err)
	}

	sdl, err := sock.GetOption(mangos.OptionSendDeadline)
	if err != nil {
		log.Fatalf("failed to get send deadline value: %s", err)
	}

	rq, err := sock.GetOption(mangos.OptionReadQLen)
	if err != nil {
		log.Fatalf("failed to get the read-queue length %s", err)
	}

	wq, err := sock.GetOption(mangos.OptionWriteQLen)
	if err != nil {
		log.Fatalf("failed to get the write-queue length %s", err)
	}
	opts := fmt.Sprintf("rx/tx deadline %s/%s, rx/tx qlen %d/%d", rdl, sdl, rq, wq)

	// retry time is the interval between sends, it is only supported on the REQ socket
	ret, err := sock.GetOption(mangos.OptionRetryTime)
	if err == nil {
		opts += fmt.Sprintf(", retry %s", ret)
	} else if err != mangos.ErrBadOption {
		log.Fatalf("failed to get retry interval value: %s", err)
	}

	reconn, err := sock.GetOption(mangos.OptionReconnectTime)
	if err != nil {
		log.Fatalf("failed to get reconnect time value: %s", err)
	}
	reconnMax, err := sock.GetOption(mangos.OptionMaxReconnectTime)
	if err != nil {
		log.Fatalf("failed to get maximum reconnect time value: %s", err)
	}
	return fmt.Sprintf("reconn/max %s/%s, %s", reconn, reconnMax, opts)
}
