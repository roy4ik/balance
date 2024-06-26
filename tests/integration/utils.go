//go:build integration
// +build integration

package integration

import (
	api "balance/gen"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func newApiClient(serverAddress string, port string) (api.BalanceClient, error) {
	conn, err := grpc.NewClient(serverAddress+":"+port, grpc.WithTransportCredentials(insecure.NewCredentials()))
	return api.NewBalanceClient(conn), err
}
