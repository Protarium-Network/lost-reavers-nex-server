package nex

import (
	nexgo "github.com/PretendoNetwork/nex-go/v2"
	"github.com/PretendoNetwork/nex-go/v2/types"
)

// pong builds a handler for a method whose only job is to prove the server is
// alive. The Bool is not optional: a caller reads exactly one boolean out of
// the response, so an empty body reads past the end of the parameter buffer.
func pong(protocolID uint16, methodID uint32) func(err error, packet nexgo.PacketInterface, callID uint32) (*nexgo.RMCMessage, *nexgo.Error) {
	return func(err error, packet nexgo.PacketInterface, callID uint32) (*nexgo.RMCMessage, *nexgo.Error) {
		if err != nil {
			return nil, nexgo.NewError(nexgo.ResultCodes.Core.InvalidArgument, err.Error())
		}

		endpoint := packet.Sender().Endpoint()

		stream := nexgo.NewByteStreamOut(endpoint.LibraryVersions(), endpoint.ByteStreamSettings())
		types.NewBool(true).WriteTo(stream)

		response := nexgo.NewRMCSuccess(endpoint, stream.Bytes())
		response.ProtocolID = protocolID
		response.MethodID = methodID
		response.CallID = callID

		return response, nil
	}
}
