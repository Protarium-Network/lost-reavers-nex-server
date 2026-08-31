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
// The access key was absent from every public source (kinnay's NEX server
// list, nex-viewer's access.txt, the wiiu.json fingerprint database, and the
// executable itself). It is now CONFIRMED: captured from a real Wii U
// console's first PRUDP packet against this server and recovered with
// Pretendo's access-key-extractor in bruteforce mode, then verified live --
// the same console completed LoginEx, RequestTicket and SecureConnection
// Register against it. See docs/ACCESS_KEY.md for the full method.
//
//	Title ID (JPN) 000500001018D900
//	Title ID (USA) 00050000101A4800
//	Title ID (EUR) 00050000101B9900
//	Game server ID 0x1018D900 (JPN, used as canonical -- see globals/config.go)
//	Access key     625f8dcc -- CONFIRMED live against a real console
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

// ServiceTokenClientIDs are the two X-Nintendo-Client-ID / client_id values
// a real console requests independent service tokens for while trying to
// reach this title, observed directly in the production account server's
// access log (neither is registered there, so both 404 without this
// server's own token endpoint intercepting them too -- see
// docs/ACCESS_KEY.md and nex/token_server.go). Nobody has documented what
// distinguishes the two; both are handled identically here.
var ServiceTokenClientIDs = []string{
	"c63718c04fd84947a6b41ffacbe53e39",
	"71a6f5d6430ea0183e3917787d717c46",
}

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
