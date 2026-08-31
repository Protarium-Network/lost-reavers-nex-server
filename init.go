package main

import (
	"crypto/rand"
	"fmt"
	"os"
	"strings"

	pb_friends "github.com/PretendoNetwork/grpc/go/friends"
	nexgo "github.com/PretendoNetwork/nex-go/v2"
	common_globals "github.com/PretendoNetwork/nex-protocols-common-go/v2/globals"
	"github.com/PretendoNetwork/plogger-go"
	"github.com/joho/godotenv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"lost-reavers/database"
	"lost-reavers/globals"
)

func init() {
	globals.Logger = plogger.NewLogger()

	// * A missing .env is fine: everything can come from the real environment.
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		globals.Logger.Warningf("Could not load .env: %v", err)
	}

	if err := globals.LoadConfig(); err != nil {
		globals.Logger.Critical(err.Error())
		os.Exit(1)
	}

	banner()

	generateKerberosPassword()
	globals.InitAccounts()

	connectAccountGRPC()
	connectFriendsGRPC()

	if err := database.Connect(globals.Settings.PostgresURI); err != nil {
		globals.Logger.Critical(err.Error())
		os.Exit(1)
	}

	if err := database.ConnectS3(); err != nil {
		globals.Logger.Critical(err.Error())
		os.Exit(1)
	}

	if globals.Settings.HealthCheckPort != 0 {
		nexgo.EnableBasicUDPHealthCheck(globals.Settings.HealthCheckPort)
		globals.Logger.Successf("UDP echo health check listening on :%d", globals.Settings.HealthCheckPort)
	}
}

func banner() {
	globals.Logger.Infof("Lost Reavers NEX server")
	globals.Logger.Infof("  Title IDs      %s (JPN) %s (USA) %s (EUR)", globals.TitleIDJPN, globals.TitleIDUSA, globals.TitleIDEUR)
	globals.Logger.Infof("  Game server ID 0x%s", globals.GameServerID)
	globals.Logger.Infof("  NEX version    %d.%d.%d", globals.Settings.NEXVersionMajor, globals.Settings.NEXVersionMinor, globals.Settings.NEXVersionPatch)
	globals.Logger.Infof("  Access key     %s", maskAccessKey(globals.Settings.AccessKey))

	if globals.Settings.LocalAuth {
		globals.Logger.Warningf("Local authentication is on: NEX passwords come from %s and login tokens are not verified", globals.Settings.SettingsFile)
	}

	if globals.Settings.AccountSecret != "" {
		globals.Logger.Warning("Derived-secret authentication is on: using the Protarium account password derivation")
	}

	if globals.Settings.SecureServerHost == "localhost" || strings.HasPrefix(globals.Settings.SecureServerHost, "127.") {
		globals.Logger.Warningf("PN_LR_SECURE_SERVER_HOST is %q. That only works if the client runs on this machine; a console needs your LAN or public address",
			globals.Settings.SecureServerHost)
	}

	if !globals.Settings.UsesFriendsGRPC() {
		globals.Logger.Info("No friends server configured. Friend-filtered matchmaking, friend leaderboards and permission=1 DataStore objects will behave as if nobody has any friends")
	}
}

// maskAccessKey shows enough of the key to eyeball it against a capture
// without dumping it in full into a log an operator might paste somewhere.
func maskAccessKey(key string) string {
	if len(key) <= 4 {
		return strings.Repeat("*", len(key))
	}

	return key[:2] + strings.Repeat("*", len(key)-4) + key[len(key)-2:]
}

func generateKerberosPassword() {
	password := make([]byte, 0x10)

	if _, err := rand.Read(password); err != nil {
		globals.Logger.Criticalf("Could not generate the server Kerberos password: %v", err)
		os.Exit(1)
	}

	globals.KerberosPassword = string(password)
}

func connectAccountGRPC() {
	if !globals.Settings.UsesAccountGRPC() {
		return
	}

	if globals.Settings.AccountGRPCAPIKey == "" {
		globals.Logger.Warning("PN_LR_ACCOUNT_GRPC_API_KEY is not set. Assuming the account gRPC API is open")
	}

	common_globals.ConnectToAccountGRPC(
		globals.Settings.AccountGRPCHost,
		uint16(globals.Settings.AccountGRPCPort),
		globals.Settings.AccountGRPCAPIKey,
	)

	globals.Logger.Successf("Using account server at %s:%d", globals.Settings.AccountGRPCHost, globals.Settings.AccountGRPCPort)
}

func connectFriendsGRPC() {
	if !globals.Settings.UsesFriendsGRPC() {
		return
	}

	address := fmt.Sprintf("%s:%d", globals.Settings.FriendsGRPCHost, globals.Settings.FriendsGRPCPort)

	connection, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		globals.Logger.Criticalf("Could not connect to the friends gRPC server: %v", err)
		os.Exit(1)
	}

	globals.GRPCFriendsClientConnection = connection
	globals.GRPCFriendsClient = pb_friends.NewFriendsClient(connection)
	globals.GRPCFriendsCommonMetadata = metadata.Pairs("X-API-Key", globals.Settings.FriendsGRPCAPIKey)

	globals.Logger.Successf("Using friends server at %s", address)
}
