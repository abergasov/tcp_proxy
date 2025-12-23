package test_utils

import (
	"context"
	"tcp_proxy/internal/config"
	"tcp_proxy/internal/logger"
	"tcp_proxy/internal/notifier"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
)

type TestContainer struct {
	Ctx context.Context
	Log logger.AppLogger
	Cfg *config.AppConfig

	SrvNotificator     notifier.Notificator
	MockController     *gomock.Controller
	SrvNotificatorMock *notifier.MockNotificator
}

func GetClean(t *testing.T) *TestContainer {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*10)
	t.Cleanup(cancel)

	cfg := LoadTestConfig(t)
	mck := gomock.NewController(t)
	mockServiceNotificator := notifier.NewMockNotificator(mck)

	appLog := logger.NewAppSLogger()
	srvNotificator := notifier.NewService(appLog, cfg.BoxName, cfg.SlackHookURL)

	return &TestContainer{
		Ctx: ctx,
		Log: appLog,
		Cfg: cfg,

		SrvNotificator:     srvNotificator,
		MockController:     mck,
		SrvNotificatorMock: mockServiceNotificator,
	}
}
