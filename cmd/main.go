package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"tcp_proxy/internal/config"
	"tcp_proxy/internal/logger"
	"tcp_proxy/internal/notifier"
	"tcp_proxy/internal/service/proxier"
)

var (
	confFile = "configs/app_conf.yml"
)

func main() {
	appLog := logger.NewAppSLogger()

	appLog.Info("app starting", logger.WithString("conf", confFile))
	appConf, err := config.LoadConfig(confFile)
	if err != nil {
		appLog.Fatal("unable to init config", err, logger.WithString("config", confFile))
	}
	ctx, cancel := context.WithCancel(context.Background())

	srvNotificator := notifier.NewService(appLog, appConf.BoxName, appConf.SlackHookURL)

	proxyList := make([]*proxier.Service, 0, len(appConf.ProxyList))
	for _, cfg := range appConf.ProxyList {
		srvProxy := proxier.NewService(ctx, &cfg, appLog, srvNotificator)
		go srvProxy.Start()
		proxyList = append(proxyList, srvProxy)
	}

	// register app shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c // This blocks the main thread until an interrupt is received
	cancel()
	for _, srvProxy := range proxyList {
		srvProxy.Stop()
	}
}
