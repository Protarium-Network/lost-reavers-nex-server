package globals

import (
	"strconv"

	"github.com/PretendoNetwork/nex-go/v2"
	"github.com/PretendoNetwork/nex-go/v2/types"
)

// The two special, non-user accounts every Quazal Rendez-Vous server has. The
// client knows them by name and expects these exact PIDs.
var (
	AuthenticationServerAccount *nex.Account
	SecureServerAccount         *nex.Account
)

// InitAccounts builds the special server accounts. Both share the freshly
// generated per-boot Kerberos password; it never leaves this process.
func InitAccounts() {
	AuthenticationServerAccount = nex.NewAccount(types.NewPID(1), "Quazal Authentication", KerberosPassword, false)
	SecureServerAccount = nex.NewAccount(types.NewPID(2), "Quazal Rendez-Vous", KerberosPassword, false)
}

// AccountDetailsByPID resolves a PID to a game server account. nex-go calls it
// while decrypting Kerberos tickets, so a wrong password here surfaces as
// "Invalid Kerberos checksum" rather than as a login failure.
func AccountDetailsByPID(pid types.PID) (*nex.Account, *nex.Error) {
	if pid.Equals(AuthenticationServerAccount.PID) {
		return AuthenticationServerAccount, nil
	}

	if pid.Equals(SecureServerAccount.PID) {
		return SecureServerAccount, nil
	}

	password, errorCode := PasswordFromPID(pid)
	if errorCode != 0 {
		return nil, nex.NewError(errorCode, "Failed to get NEX password for PID "+strconv.FormatUint(uint64(pid), 10))
	}

	// * For Wii U NEX accounts the username is the PID rendered as decimal.
	return nex.NewAccount(pid, strconv.FormatUint(uint64(pid), 10), password, false), nil
}

// AccountDetailsByUsername resolves the strUserName sent in Login/LoginEx.
func AccountDetailsByUsername(username string) (*nex.Account, *nex.Error) {
	if username == AuthenticationServerAccount.Username {
		return AuthenticationServerAccount, nil
	}

	if username == SecureServerAccount.Username {
		return SecureServerAccount, nil
	}

	pidInt, err := strconv.ParseUint(username, 10, 32)
	if err != nil {
		Logger.Errorf("Rejecting login for non-numeric username %q", username)
		return nil, nex.NewError(nex.ResultCodes.RendezVous.InvalidUsername, "Invalid username")
	}

	return AccountDetailsByPID(types.NewPID(pidInt))
}
