package main

import (
	"balance/internal/apiService"
	"balance/internal/tls"
	"log/slog"
	"os"
)

const DefaultSlbAddress = "0.0.0.0"

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
	creds, err := tls.GetTlsCredentials(tls.DefaultCAPath, tls.DefaultServiceCertPath, tls.DefaultServiceKeyPath)
	if err != nil {
		panic(err)
	}
	apiServer := apiService.NewApiServer(creds)
	defer apiServer.Stop()
	apiServer.Start()
	apiServer.Server.GetServiceInfo()
}
