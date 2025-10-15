package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"tcp_proxy/internal/config"
	"tcp_proxy/internal/logger"
	"tcp_proxy/internal/service/proxier"
)

var (
	confFile = flag.String("config", "configs/app_conf.yml", "Configs file path")
)

func main() {
	flag.Parse()
	appLog := logger.NewAppSLogger()

	appLog.Info("app starting", logger.WithString("conf", *confFile))
	appConf, err := config.InitConf(*confFile)
	if err != nil {
		appLog.Fatal("unable to init config", err, logger.WithString("config", *confFile))
	}
	ctx, cancel := context.WithCancel(context.Background())

	srv := proxier.NewService(ctx, appConf, appLog)
	go srv.Start()

	// register app shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c // This blocks the main thread until an interrupt is received
	cancel()
}
