package proxier

import (
	"net/http"
	"strings"
	"sync"
	"tcp_proxy/internal/entities"
	"tcp_proxy/internal/logger"
	"time"
)

var (
	mu            = &sync.Mutex{}
	eventsTracker = make(map[string]*entities.Notification, 1000)
	eventsCounter = make(map[string]int, 1000)

	DumpNotificationsInterval = time.Minute * 5
)

func (s *Service) handleHTTPNotification(l logger.AppLogger, r *http.Request, body []byte, remoteIP string) {
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

	l.Info("got http request",
		logger.WithString("key", d.Method),
		logger.WithString("payload", d.Body),
		logger.WithString("path", d.RemoteURL),
	)
	mu.Lock()
	defer mu.Unlock()

	eventsTracker[notifyID] = d
	if _, ok := eventsCounter[notifyID]; !ok {
		eventsCounter[notifyID] = 0
	}
	eventsCounter[notifyID]++
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
	mu.Lock()
	defer mu.Unlock()
	for id, event := range eventsTracker {
		if err := s.notificator.SendInfoNewRequest(event, s.destinationAddr, eventsCounter[id]); err != nil {
			s.log.Error("failed send notification", err)
		}
		delete(eventsTracker, id)
		delete(eventsCounter, id)
	}
}
