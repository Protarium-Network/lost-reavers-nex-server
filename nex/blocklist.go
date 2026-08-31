package nex

import (
	nexgo "github.com/PretendoNetwork/nex-go/v2"
	"github.com/PretendoNetwork/nex-go/v2/types"
	common_globals "github.com/PretendoNetwork/nex-protocols-common-go/v2/globals"
	match_making_types "github.com/PretendoNetwork/nex-protocols-go/v2/match-making/types"
	matchmake_extension "github.com/PretendoNetwork/nex-protocols-go/v2/matchmake-extension"

	"lost-reavers/database"
	"lost-reavers/globals"
)

// The block list methods live on MatchmakeExtension but have no common
// implementation, so they are wired up here. Leaving them unhandled would
// make the title receive NotImplemented the moment a player opens it.

func registerBlockList(protocol *matchmake_extension.Protocol) {
	protocol.SetHandlerGetMyBlockList(getMyBlockList)
	protocol.SetHandlerAddToBlockList(addToBlockList)
	protocol.SetHandlerRemoveFromBlockList(removeFromBlockList)
	protocol.SetHandlerClearMyBlockList(clearMyBlockList)
}

func getMyBlockList(err error, packet nexgo.PacketInterface, callID uint32) (*nexgo.RMCMessage, *nexgo.Error) {
	if err != nil {
		return nil, nexgo.NewError(nexgo.ResultCodes.Core.InvalidArgument, err.Error())
	}

	connection := packet.Sender()
	endpoint := connection.Endpoint()

	blocked, dbErr := database.GetBlockList(connection.PID())
	if dbErr != nil {
		globals.Logger.Errorf("Could not read the block list for PID %d: %v", connection.PID(), dbErr)
		return nil, nexgo.NewError(nexgo.ResultCodes.Core.Unknown, "Could not read the block list")
	}

	stream := nexgo.NewByteStreamOut(endpoint.LibraryVersions(), endpoint.ByteStreamSettings())
	blocked.WriteTo(stream)

	return respondWith(endpoint, stream.Bytes(), matchmake_extension.MethodGetMyBlockList, callID), nil
}

func addToBlockList(err error, packet nexgo.PacketInterface, callID uint32, lstPrincipalID types.List[types.PID]) (*nexgo.RMCMessage, *nexgo.Error) {
	if err != nil {
		return nil, nexgo.NewError(nexgo.ResultCodes.Core.InvalidArgument, err.Error())
	}

	connection := packet.Sender()

	if dbErr := database.AddToBlockList(connection.PID(), lstPrincipalID); dbErr != nil {
		globals.Logger.Errorf("Could not add to the block list for PID %d: %v", connection.PID(), dbErr)
		return nil, nexgo.NewError(nexgo.ResultCodes.Core.Unknown, "Could not update the block list")
	}

	return respondWith(connection.Endpoint(), nil, matchmake_extension.MethodAddToBlockList, callID), nil
}

func removeFromBlockList(err error, packet nexgo.PacketInterface, callID uint32, lstPrincipalID types.List[types.PID]) (*nexgo.RMCMessage, *nexgo.Error) {
	if err != nil {
		return nil, nexgo.NewError(nexgo.ResultCodes.Core.InvalidArgument, err.Error())
	}

	connection := packet.Sender()

	if dbErr := database.RemoveFromBlockList(connection.PID(), lstPrincipalID); dbErr != nil {
		globals.Logger.Errorf("Could not remove from the block list for PID %d: %v", connection.PID(), dbErr)
		return nil, nexgo.NewError(nexgo.ResultCodes.Core.Unknown, "Could not update the block list")
	}

	return respondWith(connection.Endpoint(), nil, matchmake_extension.MethodRemoveFromBlockList, callID), nil
}

func clearMyBlockList(err error, packet nexgo.PacketInterface, callID uint32) (*nexgo.RMCMessage, *nexgo.Error) {
	if err != nil {
		return nil, nexgo.NewError(nexgo.ResultCodes.Core.InvalidArgument, err.Error())
	}

	connection := packet.Sender()

	if dbErr := database.ClearBlockList(connection.PID()); dbErr != nil {
		globals.Logger.Errorf("Could not clear the block list for PID %d: %v", connection.PID(), dbErr)
		return nil, nexgo.NewError(nexgo.ResultCodes.Core.Unknown, "Could not update the block list")
	}

	return respondWith(connection.Endpoint(), nil, matchmake_extension.MethodClearMyBlockList, callID), nil
}

func respondWith(endpoint nexgo.EndpointInterface, body []byte, methodID uint32, callID uint32) *nexgo.RMCMessage {
	response := nexgo.NewRMCSuccess(endpoint, body)
	response.ProtocolID = matchmake_extension.ProtocolID
	response.MethodID = methodID
	response.CallID = callID

	return response
}

// canJoinMatchmakeSession is the default participation check plus a block
// list check in both directions.
func canJoinMatchmakeSession(manager *common_globals.MatchmakingManager, pid types.PID, session match_making_types.MatchmakeSession) *nexgo.Error {
	if nexError := common_globals.CanJoinMatchmakeSession(manager, pid, session); nexError != nil {
		return nexError
	}

	if database.IsBlockedEitherWay(pid, session.Gathering.OwnerPID) {
		globals.Logger.Infof("Refusing to put PID %d in a party with PID %d: one has blocked the other",
			pid, session.Gathering.OwnerPID)

		return nexgo.NewError(nexgo.ResultCodes.RendezVous.PermissionDenied, "Blocked player")
	}

	return nil
}
