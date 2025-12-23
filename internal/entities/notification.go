package entities

import (
	"fmt"
	"strings"
)

type Notification struct {
	RemoteIP    string
	RemoteURL   string
	Method      string
	ContentType string
	Body        string
	BodyLength  int64
}

func (d *Notification) NotifyID() string {
	return fmt.Sprintf("%s-%s-%s", strings.Split(d.RemoteIP, ":")[0], d.Method, d.RemoteURL)
}
