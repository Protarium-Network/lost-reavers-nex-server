package globals

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"strconv"

	pb "github.com/PretendoNetwork/grpc/go/account/v2"
	"github.com/PretendoNetwork/nex-go/v2"
	"github.com/PretendoNetwork/nex-go/v2/types"
	common_globals "github.com/PretendoNetwork/nex-protocols-common-go/v2/globals"
	"google.golang.org/grpc/metadata"
)

// PasswordFromPID returns the NEX password for a PID, or a NEX result code on
// failure. The password is whatever the account server handed the console
// inside its NEX token: if the two disagree, Kerberos fails and the game shows
// a connection error.
func PasswordFromPID(pid types.PID) (string, uint32) {
	if Settings.UsesAccountGRPC() {
		password, errorCode := passwordFromPIDGRPC(pid)
		if errorCode == 0 {
			return password, 0
		}

		if !Settings.LocalAuth {
			return "", errorCode
		}

		Logger.Warningf("Account gRPC lookup for %d failed, falling back to %s", pid, Settings.SettingsFile)
	}

	if Settings.AccountSecret != "" {
		return passwordFromPIDSecret(pid)
	}

	return passwordFromPIDLocal(pid)
}

// passwordFromPIDSecret mirrors the account service used by the other
// Protarium Wii U game servers. The account endpoint and this process derive
// the same NEX password without a per-user settings file.
func passwordFromPIDSecret(pid types.PID) (string, uint32) {
	secret, err := hex.DecodeString(Settings.AccountSecret)
	if err != nil || len(secret) == 0 {
		Logger.Error("PN_LR_ACCOUNT_SECRET is invalid")
		return "", nex.ResultCodes.RendezVous.InvalidPassword
	}

	pidBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(pidBytes, uint64(pid))
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(pidBytes)

	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), 0
}

func passwordFromPIDGRPC(pid types.PID) (string, uint32) {
	ctx := metadata.NewOutgoingContext(context.Background(), common_globals.GRPCAccountCommonMetadata)

	response, err := common_globals.GRPCAccountClient.GetNEXPassword(ctx, &pb.GetNEXPasswordRequest{Pid: uint32(pid)})
	if err != nil {
		Logger.Errorf("Account gRPC GetNEXPassword failed: %v", err)
		return "", nex.ResultCodes.RendezVous.InvalidUsername
	}

	if response.Password == "" {
		Logger.Errorf("Account server returned an empty NEX password for PID %d", pid)
		return "", nex.ResultCodes.RendezVous.InvalidPassword
	}

	return response.Password, 0
}

// localAccount mirrors one entry of nex-viewer's settings.json, so the same
// file can be pointed at both tools.
type localAccount struct {
	Platform string  `json:"platform"`
	Username string  `json:"username"`
	PID      float64 `json:"pid"`
	Password string  `json:"password"`
}

type localSettings struct {
	Accounts []localAccount `json:"accounts"`
}

// passwordFromPIDLocal reads the password out of the local settings file. The
// file is re-read on every lookup so accounts can be added without a restart.
func passwordFromPIDLocal(pid types.PID) (string, uint32) {
	if !Settings.LocalAuth {
		return "", nex.ResultCodes.RendezVous.InvalidUsername
	}

	file, err := os.ReadFile(Settings.SettingsFile)
	if err != nil {
		Logger.Errorf("Could not read %s: %v", Settings.SettingsFile, err)
		return "", nex.ResultCodes.RendezVous.InvalidUsername
	}

	var data localSettings
	if err := json.Unmarshal(file, &data); err != nil {
		Logger.Errorf("Could not parse %s: %v", Settings.SettingsFile, err)
		return "", nex.ResultCodes.RendezVous.InvalidUsername
	}

	username := strconv.FormatUint(uint64(pid), 10)

	for _, account := range data.Accounts {
		if account.Username != username && uint64(account.PID) != uint64(pid) {
			continue
		}

		if account.Password == "" {
			Logger.Errorf("Account %s in %s has an empty password", username, Settings.SettingsFile)
			return "", nex.ResultCodes.RendezVous.InvalidPassword
		}

		Logger.Infof("Using local account details for %s", username)

		return account.Password, 0
	}

	Logger.Errorf("No account for PID %s in %s. Add it, or point the server at an account server", username, Settings.SettingsFile)

	return "", nex.ResultCodes.RendezVous.InvalidUsername
}
