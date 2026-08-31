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
// Title IDs and the NEX version are CONFIRMED from two independent public
// sources that agree: Pretendo's nex-viewer titleids.txt, and kinnay's Wii U
// title library database (https://kinnay.github.io/data/wiiu.json), which
// fingerprints the exact NEX/PIA/etc. library versions linked into a game's
// executable. The latter also confirms which NEX library modules the retail
// JPN build (WUP-P-APHJ, version 48, the latest of the four JPN patches
// listed) links: nex, nexrk (Ranking), nexds (DataStore), nexmm (MatchMaking),
// nexms (Messaging) -- all at 3.10.0 -- and notably NOT nexut (Utility). That
// absence is the same kind of signal that told Mario Tennis: Ultra Smash's
// server Ranking/Utility were dead weight, just pointing the other way here:
// Utility is registered anyway, defensively, the same reasoning Ultra Smash
// applies to its own unconfirmed protocols (see nex/register_secure_protocols.go).
//
// The access key is the one thing still NOT public anywhere: absent from
// kinnay's NEX server list (https://kinnay.github.io/view.html?page=nexwiiu),
// nex-viewer's access.txt, and the wiiu.json fingerprint database above (which
// records library versions, not credentials). See docs/ACCESS_KEY.md.
//
//	Title ID (JPN) 000500001018D900
//	Title ID (USA) 00050000101A4800
//	Title ID (EUR) 00050000101B9900
//	Game server ID 0x1018D900 (JPN, used as canonical -- see globals/config.go)
//	Access key     UNKNOWN -- must be captured, see docs/ACCESS_KEY.md
//	NEX version    3.10.0 -- CONFIRMED via kinnay's wiiu.json
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
	ServerBuildString = "branch:origin/release/ngs/3.10.x.200x build:3_10_22_2006_0"
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
