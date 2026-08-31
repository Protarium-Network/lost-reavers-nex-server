// Package globals holds the process-wide state shared by the authentication
// and secure servers.
package globals

import (
	"database/sql"

	"github.com/PretendoNetwork/nex-go/v2"
	common_globals "github.com/PretendoNetwork/nex-protocols-common-go/v2/globals"
	"github.com/PretendoNetwork/plogger-go"
)

// Game constants for Lost Reavers (Wii U, Bandai Namco / ACQUIRE, 2015-2019).
//
// Title IDs are confirmed from Pretendo's nex-viewer titleids.txt (also
// present in the game's own executable strings). The access key and exact
// NEX build are NOT confirmed anywhere public as of writing: Lost Reavers is
// missing from both kinnay's NEX server list
// (https://kinnay.github.io/view.html?page=nexwiiu) and nex-viewer's
// access.txt. See docs/ACCESS_KEY.md for how to capture and confirm them.
//
//	Title ID (JPN) 000500001018D900
//	Title ID (USA) 00050000101A4800
//	Title ID (EUR) 00050000101B9900
//	Game server ID 0x1018D900 (JPN, used as canonical -- see globals/config.go)
//	Access key     UNKNOWN -- must be captured, see docs/ACCESS_KEY.md
//	NEX version    UNKNOWN -- best guess 3.9.0 based on release timing (Dec 2015 JP),
//	               same era as Mario Tennis: Ultra Smash (3.9.19) and Pokken (3.10.22).
//	               Override with PN_LR_NEX_VERSION_* once a capture confirms it.
const (
	TitleIDJPN = "000500001018D900"
	TitleIDUSA = "00050000101A4800"
	TitleIDEUR = "00050000101B9900"

	// GameServerID is the NEX game server ID, as sent by the console when it
	// requests a NEX token from the account server. It is rendered as eight
	// hex digits in requests, matching the JPN title ID's low 32 bits -- the
	// same convention Mario Tennis: Ultra Smash's server ID follows.
	GameServerID = "1018D900"

	// ServerBuildString is echoed back to the client in Login/LoginEx as
	// strReturnMsg. Titles only log it; the exact retail value is unknown, so
	// this is a plausible placeholder rather than a captured one.
	ServerBuildString = "branch:origin/release/ngs/3.9.x.200x build:3_9_19_2005_0"
)

var Logger *plogger.Logger

// KerberosPassword is the shared secret of the two special server accounts. It
// is regenerated on every boot; nothing outside this process needs to know it.
var KerberosPassword = "password"

var (
	AuthenticationServer   *nex.PRUDPServer
	AuthenticationEndpoint *nex.PRUDPEndPoint

	SecureServer   *nex.PRUDPServer
	SecureEndpoint *nex.PRUDPEndPoint
)

// Postgres backs matchmaking, unique IDs, leaderboards and DataStore metadata.
var Postgres *sql.DB

var (
	MatchmakingManager *common_globals.MatchmakingManager
	UtilityManager     *common_globals.UtilityManager
)
