package nex

import (
	"fmt"
	"sync"

	nexgo "github.com/PretendoNetwork/nex-go/v2"
	protocols_globals "github.com/PretendoNetwork/nex-protocols-go/v2/globals"

	"lost-reavers/globals"
)

// protocolNames maps NEX protocol IDs to readable names for the log. IDs are
// reused across protocol families; the names below are the ones that apply to
// a stock Wii U title of this era.
var protocolNames = map[uint16]string{
	0x01: "RemoteLogDevice",
	0x03: "NATTraversal",
	0x0A: "TicketGranting",
	0x0B: "SecureConnection",
	0x0E: "Notifications",
	0x12: "Health",
	0x13: "Monitoring",
	0x14: "Friends",
	0x15: "MatchMaking",
	0x17: "Messaging",
	0x18: "PersistentStore",
	0x19: "AccountManagement",
	0x1B: "MessageDelivery",
	0x32: "MatchMakingExt",
	0x64: "NintendoNotificationEvent",
	0x65: "Friends3DS",
	0x66: "FriendsWiiU",
	0x6D: "MatchmakeExtension",
	0x6E: "Utility",
	0x70: "Ranking",
	0x73: "DataStore",
	0x74: "Debug",
	0x75: "Subscription",
	0x76: "Rating",
	0x77: "ServiceItem",
	0x78: "MatchmakeReferee",
	0x7A: "Ranking2",
	0x7B: "AAUser",
	0x7C: "Screening",
}

// ProtocolName renders a protocol ID for humans.
func ProtocolName(id uint16) string {
	if name, ok := protocolNames[id]; ok {
		return fmt.Sprintf("%s (%#x)", name, id)
	}

	return fmt.Sprintf("Unknown (%#x)", id)
}

// tracer logs every RMC request an endpoint receives and answers the ones no
// protocol is registered for, so a missing protocol is a loud NotImplemented
// instead of the console waiting forever for a reply that never comes.
type tracer struct {
	label      string
	registered map[uint16]struct{}
	warned     map[uint16]struct{}
	mutex      sync.Mutex
}

func newTracer(label string) *tracer {
	return &tracer{
		label:      label,
		registered: make(map[uint16]struct{}),
		warned:     make(map[uint16]struct{}),
	}
}

func (t *tracer) register(ids ...uint16) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	for _, id := range ids {
		t.registered[id] = struct{}{}
	}
}

// attachLogging installs the request log. Call it before registering any
// protocol so the log line comes out before the reply.
func (t *tracer) attachLogging(endpoint *nexgo.PRUDPEndPoint) {
	endpoint.OnData(func(packet nexgo.PacketInterface) {
		request := packet.RMCMessage()
		if request == nil || !request.IsRequest {
			return
		}

		if globals.Settings.LogRMC {
			globals.Logger.Infof("[%s] %s method %#x from PID %d (call %d)",
				t.label, ProtocolName(request.ProtocolID), request.MethodID, packet.Sender().PID(), request.CallID)
		}
	})

	endpoint.OnError(func(err *nexgo.Error) {
		globals.Logger.Errorf("[%s] %v", t.label, err)
	})
}

// attachFallback installs the catch-all. It must be registered AFTER every
// protocol, since nex-go runs data handlers in registration order and this one
// only acts on what the others left alone.
func (t *tracer) attachFallback(endpoint *nexgo.PRUDPEndPoint) {
	endpoint.OnData(func(packet nexgo.PacketInterface) {
		request := packet.RMCMessage()
		if request == nil || !request.IsRequest {
			return
		}

		t.mutex.Lock()

		_, handled := t.registered[request.ProtocolID]
		_, alreadyWarned := t.warned[request.ProtocolID]

		if !handled && !alreadyWarned {
			t.warned[request.ProtocolID] = struct{}{}
		}

		t.mutex.Unlock()

		if handled {
			return
		}

		if !alreadyWarned {
			globals.Logger.Criticalf(
				"[%s] The title asked for %s, which this server does not implement. "+
					"Answering NotImplemented. If online play misbehaves, this protocol ID is why -- please report it.",
				t.label, ProtocolName(request.ProtocolID))
		}

		protocols_globals.RespondError(packet, request.ProtocolID,
			nexgo.NewError(nexgo.ResultCodes.Core.NotImplemented,
				ProtocolName(request.ProtocolID)+" is not implemented by this server"))
	})
}
