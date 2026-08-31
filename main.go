// Command lost-reavers is a replacement NEX game server for Lost Reavers
// (Wii U, Bandai Namco / ACQUIRE, 2015-2019).
//
// It runs the two PRUDP servers the title expects -- an authentication server
// that hands out Kerberos tickets, and a secure server that does matchmaking,
// NAT traversal, DataStore and leaderboards.
package main

import (
	"sync"

	"lost-reavers/nex"
)

func main() {
	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		defer wg.Done()
		nex.StartAuthenticationServer()
	}()

	go func() {
		defer wg.Done()
		nex.StartSecureServer()
	}()

	wg.Wait()
}
