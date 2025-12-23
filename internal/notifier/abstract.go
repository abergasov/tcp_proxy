package notifier

import (
	"tcp_proxy/internal/entities"
	"time"
)

type Object struct {
	Error     error  `json:"error"`
	QuotaText string `json:"quota_text"`
}

//go:generate mockgen -source=abstract.go -destination=notificator_mock.go -package=notifier
type Notificator interface {
	SendInfoMessage(message string, args ...string) error
	SendTaskErrMessage(service string, startedAt, finishedAt time.Time, message string, errs ...Object) error
	SendInfoNewRequest(n *entities.Notification, destination string, counts int) error
	SendInfoNewGRPCRequest(remoteIP, destination string) error
}
