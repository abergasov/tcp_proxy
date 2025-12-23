package entities

import "fmt"

type Notification struct {
	RemoteIP    string
	RemoteURL   string
	Method      string
	ContentType string
	Body        string
	BodyLength  int64
}

func (d *Notification) NotifyID() string {
	return fmt.Sprintf("%s-%s-%s", d.RemoteURL, d.Method, d.RemoteIP)
}
