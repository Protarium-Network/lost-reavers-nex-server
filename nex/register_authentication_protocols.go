package nex

import (
	nexgo "github.com/PretendoNetwork/nex-go/v2"
	"github.com/PretendoNetwork/nex-go/v2/constants"
	"github.com/PretendoNetwork/nex-go/v2/types"
	common_ticket_granting "github.com/PretendoNetwork/nex-protocols-common-go/v2/ticket-granting"
	ticket_granting "github.com/PretendoNetwork/nex-protocols-go/v2/ticket-granting"

	"lost-reavers/globals"
)

// registerAuthenticationProtocols wires up the single protocol the login
// server speaks: TicketGranting (0x0A) -- Login / LoginEx / RequestTicket.
func registerAuthenticationProtocols() {
	ticketGrantingProtocol := ticket_granting.NewProtocol()
	globals.AuthenticationEndpoint.RegisterServiceProtocol(ticketGrantingProtocol)
	authTracer.register(ticket_granting.ProtocolID)

	commonTicketGranting := common_ticket_granting.NewCommonProtocol(ticketGrantingProtocol)

	secureStationURL := types.NewStationURL("")
	secureStationURL.SetURLType(constants.StationURLPRUDPS)
	secureStationURL.SetAddress(globals.Settings.SecureServerHost)
	secureStationURL.SetPortNumber(uint16(globals.Settings.SecureServerPort))
	secureStationURL.SetConnectionID(1)
	secureStationURL.SetPrincipalID(types.NewPID(2))
	secureStationURL.SetStreamID(1)
	secureStationURL.SetStreamType(constants.StreamTypeRVSecure)
	secureStationURL.SetType(uint8(constants.StationURLFlagPublic))

	commonTicketGranting.SecureStationURL = secureStationURL
	commonTicketGranting.SecureServerAccount = globals.SecureServerAccount
	commonTicketGranting.BuildName = types.NewString(globals.ServerBuildString)

	// * Login (0x1) is the unauthenticated variant. A NEX 3.x title always
	// * calls LoginEx, so disabling it costs nothing and closes the door on
	// * trivial account spoofing.
	commonTicketGranting.DisableInsecureLogin()

	if globals.Settings.UsesAccountGRPC() {
		commonTicketGranting.ConfigurePNValidation([]string{globals.GameServerID})
	} else {
		commonTicketGranting.ValidateLoginData = func(pid types.PID, loginData types.DataHolder) *nexgo.Error {
			return nil
		}
	}
}
