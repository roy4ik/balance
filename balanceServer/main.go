package main

import (
	"balance/internal/apiService"
	"balance/internal/balanceService"
	"balance/internal/tls"
	"log/slog"
	"os"
)

const ApiPort = "50051"

func main() {
	logHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: true,
	})
	slog.SetDefault(slog.New(logHandler))
	defer func() {
		if err := recover(); err != nil {
			slog.Error("Program exited with an unexpected error: %s", err)
			os.Exit(1)
		}
	}()
	serverCreds, err := tls.GetTlsCredentials(tls.DefaultCAPath, tls.DefaultServiceCertPath, tls.DefaultServiceKeyPath)
	if err != nil {
		panic(err)
	}

	slbServiceImpl := balanceService.NewBalanceService()
	slbServer := apiService.NewApiServer(serverCreds, slbServiceImpl, ApiPort)
	defer slbServer.Stop()
	go slbServer.Start()

	clientCreds, err := tls.GetTlsCredentials(tls.DefaultCAPath, tls.DefaultClientCertPath, tls.DefaultClientKeyPath)
	if err != nil {
		panic(err)
	}
	slbApiClient, err := apiService.NewApiClient(clientCreds, apiService.DefaultAddress, ApiPort)
	if err != nil {
		slog.Error(err.Error())
		panic(err)
	}
	apiServerImpl := &apiService.BalanceServer{Client: slbApiClient}
	apiServer := apiService.NewApiServer(serverCreds, apiServerImpl, apiService.DefaultApiPort)
	defer apiServer.Stop()
	apiServer.Start()
}
