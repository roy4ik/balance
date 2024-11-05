package main

import (
	apiService "balance/services/api_service"
	"log/slog"
	"os"
)

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
	apiServer := apiService.NewApiServer()
	defer apiServer.Stop()
	apiServer.Start()
	apiServer.Server.GetServiceInfo()
}
