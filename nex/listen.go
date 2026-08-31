package nex

import (
	"fmt"
	"net"
	"os"

	"lost-reavers/globals"
)

// claimPort proves the UDP port is free, then releases it so the server can
// bind it for real. nex-go's Listen panics from inside a goroutine on a bind
// failure, which takes the whole process down with a stack trace instead of
// an explanation; checking first turns that into one clear sentence.
func claimPort(name string, port int) {
	address, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", port))
	if err != nil {
		globals.Logger.Criticalf("%s: :%d is not a usable UDP address: %v", name, port, err)
		os.Exit(1)
	}

	socket, err := net.ListenUDP("udp", address)
	if err != nil {
		globals.Logger.Criticalf("%s: cannot listen on UDP :%d: %v", name, port, err)
		globals.Logger.Criticalf("Another copy of this server, or another program, is probably already using it")
		os.Exit(1)
	}

	if err := socket.Close(); err != nil {
		globals.Logger.Warningf("%s: could not release the probe socket on :%d: %v", name, port, err)
	}
}
