package nex

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"net/http"

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
