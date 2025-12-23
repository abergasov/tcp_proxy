package proxier

import (
	"net/http"
	"strings"
	"tcp_proxy/internal/entities"
	"tcp_proxy/internal/logger"
	"time"
)

var (
	DumpNotificationsInterval = time.Minute * 5
)

func (s *Service) handleHTTPNotification(r *http.Request, body []byte, remoteIP string) {
	bodyStr := string(body)
	if len(body) > 1024 {
		b := append(body[:1024], []byte("…<truncated>")...)
		bodyStr = strings.ReplaceAll(string(b), "```", "`\u200b``")
	}
	d := &entities.Notification{
		RemoteIP:    remoteIP,
		RemoteURL:   r.URL.String(),
		Method:      r.Method,
		ContentType: r.Header.Get("Content-Type"),
		BodyLength:  int64(len(body)),
		Body:        bodyStr,
	}
	notifyID := d.NotifyID()
	s.mu.Lock()
	defer s.mu.Unlock()

	s.eventsTracker[notifyID] = d
	if _, ok := s.eventsCounter[notifyID]; !ok {
		s.eventsCounter[notifyID] = 0
	}
	s.eventsCounter[notifyID]++
}

func (s *Service) bgDumpNotifications() {
	ticker := time.NewTicker(DumpNotificationsInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.dumpNotifications()
		}
	}
}

func (s *Service) dumpNotifications() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, event := range s.eventsTracker {
		if err := s.notificator.SendInfoNewRequest(event, s.destinationAddr, s.eventsCounter[id]); err != nil {
			s.log.Error("failed send notification", err)
		}
		s.log.Info("got http request",
			logger.WithString("key", event.Method),
			logger.WithString("payload", event.Body),
			logger.WithString("path", event.RemoteURL),
			logger.WithInt("count", s.eventsCounter[id]),
			logger.WithString("remote_ip", event.RemoteIP),
		)
		delete(s.eventsTracker, id)
		delete(s.eventsCounter, id)
	}
}
