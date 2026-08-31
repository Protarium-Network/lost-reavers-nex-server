package nex

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"time"

	"lost-reavers/globals"
)

// nexTokenResponse mirrors the shape documented at
// https://github.com/kinnay/NintendoClients/wiki/Account-Server
// (GET /v1/api/provider/nex_token/@me) and PretendoNetwork/account's own
// implementation (src/services/nnas/routes/provider.ts).
type nexTokenResponse struct {
	XMLName     xml.Name `xml:"nex_token"`
	Host        string   `xml:"host"`
	NEXPassword string   `xml:"nex_password"`
	PID         uint64   `xml:"pid"`
	Port        int      `xml:"port"`
	Token       string   `xml:"token"`
}

// StartTokenServer runs a tiny, local-only HTTP endpoint answering the one
// account-server call this title needs that a shared account server has no
// entry for: GET /v1/api/provider/nex_token/@me?game_server_id=1018D900.
//
// It is meant to sit behind a reverse proxy path/query matcher (see
// docs/ACCESS_KEY.md) so that everything else in the login flow -- NNID
// OAuth, service tokens -- still goes through the real account server with a
// real account, and only this one game-specific call is answered locally.
// That is a deliberately narrow scope: this process has no idea whether the
// caller's OAuth token or service token were ever valid, because it never
// sees them. Binding to loopback and reaching it only through the reverse
// proxy's own path match is what keeps that acceptable.
func StartTokenServer() {
	if globals.Settings.TokenServerPort == 0 {
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/api/provider/nex_token/@me", handleNEXToken)
	mux.HandleFunc("/v1/api/provider/service_token/@me", handleServiceToken)

	address := fmt.Sprintf("%s:%d", globals.Settings.TokenServerHost, globals.Settings.TokenServerPort)

	globals.Logger.Successf("Token server listening on %s (nex_token for game_server_id=%s only)", address, globals.GameServerID)

	if err := http.ListenAndServe(address, mux); err != nil {
		globals.Logger.Criticalf("Token server failed: %v", err)
	}
}

func handleNEXToken(w http.ResponseWriter, r *http.Request) {
	gameServerID := r.URL.Query().Get("game_server_id")
	if gameServerID != globals.GameServerID {
		// * This endpoint should only ever be reached via the reverse proxy's
		// * own path+query match, so this branch is a configuration mismatch,
		// * not normal traffic -- log it plainly rather than guessing.
		globals.Logger.Warningf("Token server got game_server_id=%q, expected %q -- check the reverse proxy's query matcher",
			gameServerID, globals.GameServerID)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	token := make([]byte, 36)
	if _, err := rand.Read(token); err != nil {
		globals.Logger.Errorf("Token server: could not generate a token: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	response := nexTokenResponse{
		Host:        globals.Settings.SecureServerHost,
		NEXPassword: globals.Settings.NEXTokenPassword,
		PID:         globals.Settings.NEXTokenPID,
		Port:        globals.Settings.AuthenticationServerPort,
		Token:       base64.StdEncoding.EncodeToString(token),
	}

	globals.Logger.Infof("Token server: issuing a NEX token for PID %d, pointing at %s:%d",
		response.PID, response.Host, response.Port)

	w.Header().Set("Content-Type", "text/xml")
	w.WriteHeader(http.StatusOK)

	fmt.Fprint(w, xml.Header)
	_ = xml.NewEncoder(w).Encode(response)
}

// serviceTokenResponse mirrors PretendoNetwork/account's GET
// /provider/service_token/@me (src/services/nnas/routes/provider.ts).
type serviceTokenResponse struct {
	XMLName xml.Name `xml:"service_token"`
	Token   string   `xml:"token"`
}

// handleServiceToken answers GET /v1/api/provider/service_token/@me for the
// client_id values Lost Reavers itself requests (see
// globals.ServiceTokenClientIDs), which 404 on the real account server
// because that client_id was never registered there -- the console gives up
// on this title entirely at that point, without ever asking for a NEX token.
//
// The response format is PretendoNetwork/account's createServiceToken
// (src/util.ts): a 28-byte struct (pid, title_id, issued, expires, all
// big-endian) plus a 32-byte HMAC-SHA256, base64-encoded. The console can't
// verify the signature -- only the real game backend holds that per-title
// secret -- so a made-up key is fine; what matters is the struct being the
// right shape so client-side parsing doesn't choke on it.
func handleServiceToken(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("client_id")
	if !slices.Contains(globals.ServiceTokenClientIDs, clientID) {
		globals.Logger.Warningf("Token server got service_token client_id=%q, none of the ones this title is known to request -- check globals.ServiceTokenClientIDs",
			clientID)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	titleID, err := strconv.ParseUint(globals.TitleIDEUR, 16, 64)
	if err != nil {
		globals.Logger.Errorf("Token server: TitleIDEUR does not parse as hex: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	now := time.Now()
	expires := now.Add(time.Hour)

	data := make([]byte, 28)
	binary.BigEndian.PutUint32(data[0:4], uint32(globals.Settings.NEXTokenPID))
	binary.BigEndian.PutUint64(data[4:12], titleID)
	binary.BigEndian.PutUint64(data[12:20], uint64(now.UnixMilli()))
	binary.BigEndian.PutUint64(data[20:28], uint64(expires.UnixMilli()))

	mac := hmac.New(sha256.New, []byte("lost-reavers-capture-fake-server-key"))
	mac.Write(data)
	signature := mac.Sum(nil)

	response := serviceTokenResponse{
		Token: base64.StdEncoding.EncodeToString(append(data, signature...)),
	}

	globals.Logger.Infof("Token server: issuing a service token for client_id=%s, PID %d", clientID, globals.Settings.NEXTokenPID)

	w.Header().Set("Content-Type", "text/xml")
	w.WriteHeader(http.StatusOK)

	fmt.Fprint(w, xml.Header)
	_ = xml.NewEncoder(w).Encode(response)
}
