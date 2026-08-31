package nex

import (
	nexgo "github.com/PretendoNetwork/nex-go/v2"
	common_globals "github.com/PretendoNetwork/nex-protocols-common-go/v2/globals"

	"lost-reavers/globals"
)

var secureTracer = newTracer("secure")

// StartSecureServer runs the secure (game) server: registering the console's
// station URLs, matchmaking, NAT traversal, DataStore and leaderboards. This
// call blocks.
func StartSecureServer() {
	globals.SecureServer = nexgo.NewPRUDPServer()

	globals.SecureServer.ByteStreamSettings.UseStructureHeader = true
	globals.SecureServer.LibraryVersions.SetDefault(
		nexgo.NewLibraryVersion(globals.Settings.NEXVersionMajor, globals.Settings.NEXVersionMinor, globals.Settings.NEXVersionPatch))
	globals.SecureServer.AccessKey = globals.Settings.AccessKey

	globals.SecureEndpoint = nexgo.NewPRUDPEndPoint(1)
	globals.SecureEndpoint.IsSecureEndPoint = true
	globals.SecureEndpoint.ServerAccount = globals.SecureServerAccount
	globals.SecureEndpoint.AccountDetailsByPID = globals.AccountDetailsByPID
	globals.SecureEndpoint.AccountDetailsByUsername = globals.AccountDetailsByUsername

	// * A dungeon run is a lot longer than a tennis match. Give consoles more
	// * room before a silent lobby looks like a dead connection.
	globals.SecureEndpoint.DefaultStreamSettings.MaxSilenceTime = 30000

	globals.SecureServer.BindPRUDPEndPoint(globals.SecureEndpoint)

	globals.SecureEndpoint.EnableVerboseRMC(globals.Settings.VerboseRMC)

	secureTracer.attachLogging(globals.SecureEndpoint)

	globals.MatchmakingManager = common_globals.NewMatchmakingManager(globals.SecureEndpoint, globals.Postgres)
	globals.MatchmakingManager.GetUserFriendPIDs = globals.GetUserFriendPIDs

	globals.UtilityManager = common_globals.NewUtilityManager(globals.SecureEndpoint, globals.Postgres)

	registerSecureProtocols()

	secureTracer.attachFallback(globals.SecureEndpoint)

	claimPort("Secure server", globals.Settings.SecureServerPort)

	globals.Logger.Successf("Secure server listening on UDP :%d (advertised as %s:%d)",
		globals.Settings.SecureServerPort, globals.Settings.SecureServerHost, globals.Settings.SecureServerPort)

	globals.SecureServer.Listen(globals.Settings.SecureServerPort)
}
