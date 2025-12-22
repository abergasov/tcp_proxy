package notifier

import (
	"net/http"
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
	SendInfoNewRequest(r *http.Request, body []byte, remoteIP, destination string) error
	SendInfoNewGRPCRequest(remoteIP, destination string) error
}
