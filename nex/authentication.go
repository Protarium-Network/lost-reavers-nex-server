// Package nex starts and configures the two PRUDP servers this title needs.
package nex

import (
	nexgo "github.com/PretendoNetwork/nex-go/v2"

	"lost-reavers/globals"
)

var authTracer = newTracer("auth")

// StartAuthenticationServer runs the authentication (login) server. This call
// blocks.
func StartAuthenticationServer() {
	globals.AuthenticationServer = nexgo.NewPRUDPServer()

	globals.AuthenticationServer.ByteStreamSettings.UseStructureHeader = true
	globals.AuthenticationServer.LibraryVersions.SetDefault(
		nexgo.NewLibraryVersion(globals.Settings.NEXVersionMajor, globals.Settings.NEXVersionMinor, globals.Settings.NEXVersionPatch))
	globals.AuthenticationServer.AccessKey = globals.Settings.AccessKey

	globals.AuthenticationEndpoint = nexgo.NewPRUDPEndPoint(1)
	globals.AuthenticationEndpoint.ServerAccount = globals.AuthenticationServerAccount
	globals.AuthenticationEndpoint.AccountDetailsByPID = globals.AccountDetailsByPID
	globals.AuthenticationEndpoint.AccountDetailsByUsername = globals.AccountDetailsByUsername
	globals.AuthenticationServer.BindPRUDPEndPoint(globals.AuthenticationEndpoint)

	globals.AuthenticationEndpoint.EnableVerboseRMC(globals.Settings.VerboseRMC)

	authTracer.attachLogging(globals.AuthenticationEndpoint)

	registerAuthenticationProtocols()

	authTracer.attachFallback(globals.AuthenticationEndpoint)

	claimPort("Authentication server", globals.Settings.AuthenticationServerPort)

	globals.Logger.Successf("Authentication server listening on UDP :%d", globals.Settings.AuthenticationServerPort)

	globals.AuthenticationServer.Listen(globals.Settings.AuthenticationServerPort)
}
