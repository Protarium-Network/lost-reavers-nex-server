package nex

import (
	nexgo "github.com/PretendoNetwork/nex-go/v2"
	"github.com/PretendoNetwork/nex-go/v2/types"
	common_datastore "github.com/PretendoNetwork/nex-protocols-common-go/v2/datastore"
	common_globals "github.com/PretendoNetwork/nex-protocols-common-go/v2/globals"
	common_match_making "github.com/PretendoNetwork/nex-protocols-common-go/v2/match-making"
	common_match_making_ext "github.com/PretendoNetwork/nex-protocols-common-go/v2/match-making-ext"
	common_matchmake_extension "github.com/PretendoNetwork/nex-protocols-common-go/v2/matchmake-extension"
	common_messaging "github.com/PretendoNetwork/nex-protocols-common-go/v2/messaging"
	common_nat_traversal "github.com/PretendoNetwork/nex-protocols-common-go/v2/nat-traversal"
	common_ranking "github.com/PretendoNetwork/nex-protocols-common-go/v2/ranking"
	common_secure_connection "github.com/PretendoNetwork/nex-protocols-common-go/v2/secure-connection"
	common_utility "github.com/PretendoNetwork/nex-protocols-common-go/v2/utility"
	datastore "github.com/PretendoNetwork/nex-protocols-go/v2/datastore"
	health "github.com/PretendoNetwork/nex-protocols-go/v2/health"
	match_making "github.com/PretendoNetwork/nex-protocols-go/v2/match-making"
	match_making_ext "github.com/PretendoNetwork/nex-protocols-go/v2/match-making-ext"
	matchmake_extension "github.com/PretendoNetwork/nex-protocols-go/v2/matchmake-extension"
	messaging "github.com/PretendoNetwork/nex-protocols-go/v2/messaging"
	monitoring "github.com/PretendoNetwork/nex-protocols-go/v2/monitoring"
	nat_traversal "github.com/PretendoNetwork/nex-protocols-go/v2/nat-traversal"
	ranking "github.com/PretendoNetwork/nex-protocols-go/v2/ranking"
	secure_connection "github.com/PretendoNetwork/nex-protocols-go/v2/secure-connection"
	utility "github.com/PretendoNetwork/nex-protocols-go/v2/utility"

	"lost-reavers/database"
	"lost-reavers/globals"
)

// registerSecureProtocols wires up every protocol the secure server answers.
//
// The set below is what the game's own executable proves it links, from the
// nn::nex::*DDLDeclarations and Protocol/ProtocolClient symbol names found in
// the decompressed RPX (see docs/PROTOCOLS.md for the full evidence trail):
//
//	SecureConnection   (0x0B)  register the console's station URLs
//	NATTraversal       (0x03)  hole punching between up to four consoles
//	MatchMaking        (0x15)  gathering lifecycle, host migration, session URLs
//	MatchMakingExt     (0x32)  leaving a gathering
//	MatchmakeExtension (0x6D)  create/browse/join/auto-matchmake a party, plus
//	                           communities (CreateCommunity, PersistentGathering)
//	DataStore          (0x73)  treasure/inventory objects, confirmed used --
//	                           unlike Ultra Smash, this title's DataStore
//	                           symbols (DataStorePreparePostParam,
//	                           DataStoreFileServerClient, ...) are extensive
//	Ranking            (0x70)  leaderboards
//	Messaging          (0x17)  MessagingServiceDDLDeclarations is linked in;
//	                           registered for real, not defensively
//	Utility            (0x6E)  NEX unique IDs
//	Monitoring         (0x13)  liveness ping
//	Health             (0x12)  liveness ping
//
// One confirmed protocol has NO common implementation and is intentionally
// left unregistered: NintendoNotificationEvent (0x64), used for friend
// presence broadcasts. nex-protocols-common-go has no ready-made handler for
// it, and the console will get a loud, logged NotImplemented instead of a
// silent hang -- see docs/PROTOCOLS.md.
func registerSecureProtocols() {
	registerSecureConnection()
	registerNATTraversal()
	registerMatchmaking()
	registerDataStore()
	registerRanking()
	registerMessaging()
	registerUtility()
	registerLiveness()
}

// registerSecureConnection handles Register/RegisterEx, where the console
// tells us the addresses other party members should use to reach it.
func registerSecureConnection() {
	secureProtocol := secure_connection.NewProtocol()
	globals.SecureEndpoint.RegisterServiceProtocol(secureProtocol)
	secureTracer.register(secure_connection.ProtocolID)

	commonSecure := common_secure_connection.NewCommonProtocol(secureProtocol)

	// * Which of Register/RegisterEx the title calls is unconfirmed, so both
	// * are enabled -- the same approach Mario Tennis: Ultra Smash's server
	// * uses for the same reason. The console only reaches this endpoint by
	// * presenting a Kerberos ticket it could not have decrypted without the
	// * right NEX password, so enabling insecure Register is not a hole here.
	commonSecure.EnableInsecureRegister()

	if globals.Settings.UsesAccountGRPC() {
		commonSecure.ConfigurePNValidation([]string{globals.GameServerID})
	} else {
		commonSecure.ValidateLoginData = func(pid types.PID, loginData types.DataHolder) *nexgo.Error {
			return nil
		}
	}

	commonSecure.CreateReportDBRecord = func(pid types.PID, reportID types.UInt32, reportData types.QBuffer) error {
		globals.Logger.Warningf("Player report from PID %d (report ID %d, %d bytes) discarded: this server does not store reports",
			pid, reportID, len(reportData))
		return nil
	}
}

func registerNATTraversal() {
	natTraversalProtocol := nat_traversal.NewProtocol()
	globals.SecureEndpoint.RegisterServiceProtocol(natTraversalProtocol)
	secureTracer.register(nat_traversal.ProtocolID)

	common_nat_traversal.NewCommonProtocol(natTraversalProtocol)
}

// registerMatchmaking wires the three protocols that together make up NEX
// matchmaking, including communities (CreateCommunity / PersistentGathering),
// which the RPX shows this title uses -- plausibly its four-player party hub.
func registerMatchmaking() {
	matchMakingProtocol := match_making.NewProtocol()
	globals.SecureEndpoint.RegisterServiceProtocol(matchMakingProtocol)
	secureTracer.register(match_making.ProtocolID)
	commonMatchMaking := common_match_making.NewCommonProtocol(matchMakingProtocol)
	commonMatchMaking.SetManager(globals.MatchmakingManager)
	commonMatchMaking.OnAfterGetSessionURLs = func(packet nexgo.PacketInterface, gid types.UInt32) {
		var hostPID uint64
		if err := globals.Postgres.QueryRow(`SELECT host_pid FROM matchmaking.gatherings WHERE id=$1`, uint32(gid)).Scan(&hostPID); err != nil {
			globals.Logger.Errorf("GetSessionURLs: PID %d requested gathering %d, but its host could not be read: %v",
				packet.Sender().PID(), gid, err)
			return
		}

		host := globals.SecureEndpoint.FindConnectionByPID(hostPID)
		if host == nil {
			globals.Logger.Errorf("GetSessionURLs: PID %d requested gathering %d whose host PID %d is not connected; an empty URL list was returned",
				packet.Sender().PID(), gid, hostPID)
			return
		}

		globals.Logger.Infof("GetSessionURLs: PID %d requested gathering %d; host PID %d URLs=%v",
			packet.Sender().PID(), gid, hostPID, host.StationURLs)
	}

	matchMakingExtProtocol := match_making_ext.NewProtocol()
	globals.SecureEndpoint.RegisterServiceProtocol(matchMakingExtProtocol)
	secureTracer.register(match_making_ext.ProtocolID)
	common_match_making_ext.NewCommonProtocol(matchMakingExtProtocol).SetManager(globals.MatchmakingManager)

	matchmakeExtensionProtocol := matchmake_extension.NewProtocol()
	globals.SecureEndpoint.RegisterServiceProtocol(matchmakeExtensionProtocol)
	secureTracer.register(matchmake_extension.ProtocolID)

	commonMatchmakeExtension := common_matchmake_extension.NewCommonProtocol(matchmakeExtensionProtocol)
	commonMatchmakeExtension.SetManager(globals.MatchmakingManager)
	commonMatchmakeExtension.CanJoinMatchmakeSession = canJoinMatchmakeSession

	registerMatchmakingLogging(commonMatchmakeExtension)
	registerBlockList(matchmakeExtensionProtocol)

	if !globals.Settings.AllowPublicMatchmaking {
		globals.Logger.Warning("Random matchmaking is disabled. Party creation may still work, but strangers cannot find each other")

		matchmakeExtensionProtocol.SetHandlerBrowseMatchmakeSession(nil)
		matchmakeExtensionProtocol.SetHandlerAutoMatchmakePostpone(nil)
		matchmakeExtensionProtocol.SetHandlerAutoMatchmakeWithParamPostpone(nil)
		matchmakeExtensionProtocol.SetHandlerAutoMatchmakeWithSearchCriteriaPostpone(nil)
	}
}

// registerDataStore backs treasure/inventory persistence. Object bodies live
// in an S3-compatible bucket (MinIO by default, see compose.yaml); metadata
// lives in Postgres, in database/datastore.go.
func registerDataStore() {
	dataStoreProtocol := datastore.NewProtocol()
	globals.SecureEndpoint.RegisterServiceProtocol(dataStoreProtocol)
	secureTracer.register(datastore.ProtocolID)

	commonDataStore := common_datastore.NewCommonProtocol(dataStoreProtocol)
	commonDataStore.S3Bucket = globals.Settings.S3Bucket
	commonDataStore.SetMinIOClient(database.MinIOClient)
	commonDataStore.SetDataKeyBase("objects")
	commonDataStore.SetNotifyKeyBase("notifications")
	commonDataStore.GetUserFriendPIDs = globals.GetUserFriendPIDs

	commonDataStore.InitializeObjectByPreparePostParam = database.InitializeObjectByPreparePostParam
	commonDataStore.InitializeObjectRatingWithSlot = database.InitializeObjectRatingWithSlot
	commonDataStore.GetObjectInfoByDataID = database.GetObjectInfoByDataID
	commonDataStore.GetObjectInfoByDataIDWithPassword = database.GetObjectInfoByDataIDWithPassword
	commonDataStore.GetObjectInfoByPersistenceTargetWithPassword = database.GetObjectInfoByPersistenceTargetWithPassword
	commonDataStore.GetObjectSizeByDataID = database.GetObjectSizeByDataID
	commonDataStore.GetObjectOwnerByDataID = database.GetObjectOwnerByDataID
	commonDataStore.UpdateObjectUploadCompletedByDataID = database.UpdateObjectUploadCompletedByDataID
	commonDataStore.UpdateObjectPeriodByDataIDWithPassword = database.UpdateObjectPeriodByDataIDWithPassword
	commonDataStore.UpdateObjectMetaBinaryByDataIDWithPassword = database.UpdateObjectMetaBinaryByDataIDWithPassword
	commonDataStore.UpdateObjectDataTypeByDataIDWithPassword = database.UpdateObjectDataTypeByDataIDWithPassword
	commonDataStore.DeleteObjectByDataID = database.DeleteObjectByDataID
	commonDataStore.DeleteObjectByDataIDWithPassword = database.DeleteObjectByDataIDWithPassword
	commonDataStore.GetObjectInfosByDataStoreSearchParam = database.GetObjectInfosByDataStoreSearchParam
	commonDataStore.RateObjectWithPassword = database.RateObjectWithPassword
}

// registerRanking backs the online leaderboards.
func registerRanking() {
	rankingProtocol := ranking.NewProtocol()
	globals.SecureEndpoint.RegisterServiceProtocol(rankingProtocol)
	secureTracer.register(ranking.ProtocolID)

	commonRanking := common_ranking.NewCommonProtocol(rankingProtocol)

	commonRanking.InsertRankingByPIDAndRankingScoreData = database.InsertRankingByPIDAndRankingScoreData
	commonRanking.GetRankingsAndCountByCategoryAndRankingOrderParam = database.GetRankingsAndCountByCategoryAndRankingOrderParam
	commonRanking.GetOwnRankingByCategoryAndRankingOrderParam = database.GetOwnRankingByCategoryAndRankingOrderParam
	commonRanking.GetNearbyRankingsAndCountByCategoryAndRankingOrderParam = database.GetNearbyRankingsAndCountByCategoryAndRankingOrderParam
	commonRanking.GetFriendsRankingsAndCountByCategoryAndRankingOrderParam = database.GetFriendsRankingsAndCountByCategoryAndRankingOrderParam
	commonRanking.GetNearbyFriendsRankingsAndCountByCategoryAndRankingOrderParam = database.GetNearbyFriendsRankingsAndCountByCategoryAndRankingOrderParam
	commonRanking.GetCommonData = database.GetCommonData
	commonRanking.UploadCommonData = database.UploadCommonData
}

// registerMessaging backs the Messaging protocol (mailbox-style delivery,
// e.g. friend/system notices). Unlike Ranking on Ultra Smash, this is not a
// defensive registration: MessagingServiceDDLDeclarations is linked into the
// executable, which is the same signal that told Ultra Smash's server DataStore
// was safe to skip -- here it points the other way.
func registerMessaging() {
	messagingProtocol := messaging.NewProtocol()
	globals.SecureEndpoint.RegisterServiceProtocol(messagingProtocol)
	secureTracer.register(messaging.ProtocolID)

	commonMessaging := common_messaging.NewCommonProtocol(messagingProtocol)

	messagingManager := common_globals.NewMessagingManager(globals.SecureEndpoint, globals.Postgres)
	messagingManager.MatchmakingManager = globals.MatchmakingManager

	commonMessaging.SetManager(messagingManager)
}

func registerUtility() {
	utilityProtocol := utility.NewProtocol()
	globals.SecureEndpoint.RegisterServiceProtocol(utilityProtocol)
	secureTracer.register(utility.ProtocolID)

	common_utility.NewCommonProtocol(utilityProtocol).SetManager(globals.UtilityManager)
}

func registerLiveness() {
	monitoringProtocol := monitoring.NewProtocol()
	monitoringProtocol.SetHandlerPingDaemon(pong(monitoring.ProtocolID, monitoring.MethodPingDaemon))
	globals.SecureEndpoint.RegisterServiceProtocol(monitoringProtocol)
	secureTracer.register(monitoring.ProtocolID)

	healthProtocol := health.NewProtocol()
	healthProtocol.SetHandlerPingDaemon(pong(health.ProtocolID, health.MethodPingDaemon))
	healthProtocol.SetHandlerPingDatabase(pong(health.ProtocolID, health.MethodPingDatabase))
	globals.SecureEndpoint.RegisterServiceProtocol(healthProtocol)
	secureTracer.register(health.ProtocolID)
}
