package globals

import (
	"context"

	pb "github.com/PretendoNetwork/grpc/go/friends"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

var (
	GRPCFriendsClientConnection *grpc.ClientConn
	GRPCFriendsClient           pb.FriendsClient
	GRPCFriendsCommonMetadata   metadata.MD
)

// GetUserFriendPIDs backs the generic NEX friend checks: a gathering may set a
// participation policy of "friends of the owner only", friend leaderboards
// filter on the same list, and DataStore's permission=1 ("friends only")
// objects need it too. The RPX shows Lost Reavers links nn::act's friend and
// presence code, so unlike Ultra Smash this is plausibly load-bearing --
// configure a friends server if one is available.
func GetUserFriendPIDs(pid uint32) []uint32 {
	if GRPCFriendsClient == nil {
		return []uint32{}
	}

	ctx := metadata.NewOutgoingContext(context.Background(), GRPCFriendsCommonMetadata)

	response, err := GRPCFriendsClient.GetUserFriendPIDs(ctx, &pb.GetUserFriendPIDsRequest{Pid: pid})
	if err != nil {
		Logger.Errorf("Friends gRPC GetUserFriendPIDs failed: %v", err)
		return []uint32{}
	}

	return response.Pids
}
