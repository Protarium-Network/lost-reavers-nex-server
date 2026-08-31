package nex

import (
	"fmt"
	"strings"

	nexgo "github.com/PretendoNetwork/nex-go/v2"
	"github.com/PretendoNetwork/nex-go/v2/types"
	common_matchmake_extension "github.com/PretendoNetwork/nex-protocols-common-go/v2/matchmake-extension"
	match_making_types "github.com/PretendoNetwork/nex-protocols-go/v2/match-making/types"

	"lost-reavers/globals"
)

// Nobody has published a capture of Lost Reavers' matchmaking traffic, so the
// meaning of its GameMode word and attribute words -- almost certainly
// encoding the mission/stage, difficulty and party size out of a 1-4 player
// co-op raid -- is unknown. These hooks print exactly what the title sends, so
// the first console to reach Online can have its lobby search read straight
// out of the log instead of guessed at. They only read; nothing here changes
// what the title asked for.

func registerMatchmakingLogging(common *common_matchmake_extension.CommonProtocol) {
	if !globals.Settings.LogMatchmaking {
		return
	}

	common.CleanupSearchMatchmakeSession = func(session *match_making_types.MatchmakeSession) {
		globals.Logger.Infof("Matchmaking search template:\n%s", describeSession(*session))
	}

	common.CleanupMatchmakeSessionSearchCriterias = func(criterias types.List[match_making_types.MatchmakeSessionSearchCriteria]) {
		for i := range criterias {
			criteria := criterias[i]
			globals.Logger.Infof("Matchmaking search criteria %d:\n%s", i, describeCriteria(criteria))
		}
	}

	common.OnAfterCreateMatchmakeSession = func(packet nexgo.PacketInterface, gathering match_making_types.GatheringHolder, message types.String, participationCount types.UInt16) {
		globals.Logger.Infof("PID %d created a party (%d participant(s))", packet.Sender().PID(), participationCount)
	}

	common.OnAfterAutoMatchmakePostpone = func(packet nexgo.PacketInterface, gathering match_making_types.GatheringHolder, message types.String) {
		globals.Logger.Infof("PID %d auto-matchmade into gathering %d", packet.Sender().PID(), gatheringID(gathering))
	}

	common.OnAfterAutoMatchmakeWithSearchCriteriaPostpone = func(packet nexgo.PacketInterface, criterias types.List[match_making_types.MatchmakeSessionSearchCriteria], gathering match_making_types.GatheringHolder, message types.String) {
		globals.Logger.Infof("PID %d auto-matchmade by criteria into gathering %d", packet.Sender().PID(), gatheringID(gathering))
	}

	common.OnAfterJoinMatchmakeSession = func(packet nexgo.PacketInterface, gid types.UInt32, message types.String) {
		globals.Logger.Infof("PID %d joined gathering %d", packet.Sender().PID(), gid)
	}

	common.OnAfterJoinMatchmakeSessionEx = func(packet nexgo.PacketInterface, gid types.UInt32, message types.String, dontCareMyBlockList types.Bool, participationCount types.UInt16) {
		globals.Logger.Infof("PID %d joined gathering %d with %d participant(s)", packet.Sender().PID(), gid, participationCount)
	}
}

func gatheringID(holder match_making_types.GatheringHolder) uint32 {
	if holder.Object == nil {
		return 0
	}

	if session, ok := holder.Object.(match_making_types.MatchmakeSession); ok {
		return uint32(session.Gathering.ID)
	}

	if session, ok := holder.Object.(*match_making_types.MatchmakeSession); ok {
		return uint32(session.Gathering.ID)
	}

	return 0
}

func describeSession(session match_making_types.MatchmakeSession) string {
	var b strings.Builder

	fmt.Fprintf(&b, "  GameMode            %d (%#x)\n", session.GameMode, uint32(session.GameMode))
	fmt.Fprintf(&b, "  Attributes          %s\n", describeAttributes(session.Attributes))
	fmt.Fprintf(&b, "  Participants        %d..%d\n", session.Gathering.MinimumParticipants, session.Gathering.MaximumParticipants)
	fmt.Fprintf(&b, "  MatchmakeSystemType %d\n", session.MatchmakeSystemType)
	fmt.Fprintf(&b, "  OpenParticipation   %t\n", bool(session.OpenParticipation))
	fmt.Fprintf(&b, "  ParticipationPolicy %d (argument %d)\n", session.Gathering.ParticipationPolicy, session.Gathering.PolicyArgument)
	fmt.Fprintf(&b, "  ApplicationBuffer   %d bytes\n", len(session.ApplicationBuffer))

	return strings.TrimRight(b.String(), "\n")
}

func describeCriteria(criteria match_making_types.MatchmakeSessionSearchCriteria) string {
	var b strings.Builder

	fmt.Fprintf(&b, "  GameMode            %q\n", string(criteria.GameMode))
	fmt.Fprintf(&b, "  Attribs             %s\n", describeStrings(criteria.Attribs))
	fmt.Fprintf(&b, "  Participants        %q..%q\n", string(criteria.MinParticipants), string(criteria.MaxParticipants))
	fmt.Fprintf(&b, "  MatchmakeSystemType %q\n", string(criteria.MatchmakeSystemType))
	fmt.Fprintf(&b, "  VacantOnly          %t (vacant participants %d)\n", bool(criteria.VacantOnly), criteria.VacantParticipants)
	fmt.Fprintf(&b, "  ExcludeLocked       %t\n", bool(criteria.ExcludeLocked))
	fmt.Fprintf(&b, "  SelectionMethod     %d\n", criteria.SelectionMethod)

	return strings.TrimRight(b.String(), "\n")
}

func describeAttributes(attributes types.List[types.UInt32]) string {
	if len(attributes) == 0 {
		return "(none)"
	}

	parts := make([]string, 0, len(attributes))
	for i, attribute := range attributes {
		parts = append(parts, fmt.Sprintf("[%d]=%d (%#x)", i, attribute, uint32(attribute)))
	}

	return strings.Join(parts, " ")
}

func describeStrings(values types.List[types.String]) string {
	if len(values) == 0 {
		return "(none)"
	}

	parts := make([]string, 0, len(values))
	for i, value := range values {
		parts = append(parts, fmt.Sprintf("[%d]=%q", i, string(value)))
	}

	return strings.Join(parts, " ")
}
