package notifier

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"tcp_proxy/internal/logger"
	"tcp_proxy/internal/utils"

	"time"
)

type Service struct {
	boxName      string
	log          logger.AppLogger
	slackHookURL string
}

type slackBlock struct {
	Type     string    `json:"type"`
	BlockID  string    `json:"block_id,omitempty"`
	Text     *item     `json:"text,omitempty"`
	Fields   []item    `json:"fields,omitempty"`
	Elements []element `json:"elements,omitempty"`
}

type element struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Style    string `json:"style,omitempty"`
	Elements []item `json:"elements,omitempty"`
}

type item struct {
	Type     string `json:"type"`
	Text     string `json:"text"`
	Elements []item `json:"elements,omitempty"`
}

type tableRowElement struct {
	Type  string          `json:"type"`
	Text  string          `json:"text"`
	Style map[string]bool `json:"style,omitempty"`
}

type tableElements struct {
	Type     string            `json:"type"`
	Elements []tableRowElement `json:"elements"`
}

var (
	divider = slackBlock{Type: "divider"}
)

func NewService(l logger.AppLogger, boxName, slackHookURL string) *Service {
	return &Service{
		boxName:      boxName,
		log:          l,
		slackHookURL: slackHookURL,
	}
}

func (s *Service) SendInfoNewGRPCRequest(remoteIP, destination string) error {
	return s.sendSlackMessage(map[string]any{
		"blocks": []any{
			getHeader(":eyes: observe new unsecure grpc request"),
			map[string]any{
				"type": "section",
				"fields": []any{
					slackField("From", remoteIP),
					slackField("To", destination),
				},
			},
			s.getContext(),
		},
	})
}

func (s *Service) SendInfoNewRequest(r *http.Request, body []byte, remoteIP, destination string) error {
	blocks := []any{
		getHeader(":eyes: observe new http request"),
		map[string]any{
			"type": "section",
			"fields": []any{
				slackField("From", remoteIP),
				slackField("URL", r.URL.String()),
			},
		},
	}
	if len(body) > 0 {
		blocks = append(blocks, map[string]any{
			"type": "section",
			"text": map[string]any{
				"type": "mrkdwn",
				"text": "```" + slackSafeBody(body, 1500) + "```",
			},
		})
	}
	blocks = append(blocks, s.getContextWithExtra(
		fmt.Sprintf("content-length: *%d*", len(body)),
		fmt.Sprintf("content-type: %s", r.Header.Get("content-type")),
		fmt.Sprintf("method: %s", r.Method),
		fmt.Sprintf("destination: %s", destination),
	))
	return s.sendSlackMessage(map[string]any{"blocks": blocks})
}

func (s *Service) SendTaskErrMessage(service string, startedAt, finishedAt time.Time, message string, errs ...Object) error {
	errMsg := strings.Builder{}
	errBlock := slackBlock{
		Type:     "rich_text",
		BlockID:  "block1",
		Elements: make([]element, 0, len(errs)),
	}
	if message != "" {
		errBlock.Elements = append(errBlock.Elements, element{
			Type: "rich_text_section",
			Elements: []item{
				{
					Type: "text",
					Text: message,
				},
			},
		})
	}
	prefix := false
	if len(errs) > 1 {
		prefix = true
	}
	for i := range errs {
		if errs[i].Error != nil {
			prefixText := ""
			if prefix {
				prefixText = fmt.Sprintf("error %d: ", i+1)
			}
			errBlock.Elements = append(errBlock.Elements, element{
				Type: "rich_text_section",
				Elements: []item{
					{
						Type: "text",
						Text: fmt.Sprintf("%s%s\n", prefixText, errs[i].Error.Error()),
					},
				},
			})
			errMsg.WriteString(fmt.Sprintf("error %d: %s\n", i+1, errs[i].Error.Error()))
		}
		if errs[i].QuotaText != "" {
			errBlock.Elements = append(errBlock.Elements, element{
				Type: "rich_text_preformatted",
				Elements: []item{
					{
						Type: "text",
						Text: errs[i].QuotaText,
					},
				},
			})
			errMsg.WriteString(fmt.Sprintf("Quota text: %s\n", errs[i].QuotaText))
		}
	}
	if errMsg.Len() == 0 {
		return nil
	}

	return s.sendSlackMessage(map[string][]slackBlock{
		"blocks": {
			getHeader(fmt.Sprintf(":bangbang: Service job failed: %s", service)),
			divider,
			errBlock,
			{
				Type: "section",

				Fields: []item{
					{
						Type: "mrkdwn",
						Text: "*started at*",
					},
					{
						Type: "mrkdwn",
						Text: "*finished at*",
					},
					{
						Type: "mrkdwn",
						Text: startedAt.Format(time.DateTime),
					},
					{
						Type: "mrkdwn",
						Text: fmt.Sprintf("%s (%s)", finishedAt.Format(time.DateTime), finishedAt.Sub(startedAt).String()),
					},
				},
			},
			divider,
			s.getContext(),
		},
	})
}

func (s *Service) SendInfoMessage(message string, args ...string) error {
	return s.sendInfoMessage(message, args...)
}

func (s *Service) sendInfoMessage(message string, args ...string) error {
	blocks := make([]slackBlock, 0, len(args)+3)
	blocks = append(blocks, getHeader(fmt.Sprintf(":warning: %s", message)))
	blocks = append(blocks, divider)
	if len(args) > 0 {
		for i := range args {
			blocks = append(blocks, slackBlock{
				Type: "section",
				Text: &item{
					Type: "mrkdwn",
					Text: args[i],
				},
			})
		}
		blocks = append(blocks, divider)
	}
	blocks = append(blocks, s.getContext())
	return s.sendSlackMessage(map[string][]slackBlock{"blocks": blocks})
}

func (s *Service) sendSlackMessage(payload any) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, code, err := utils.PostCurl[any](ctx, s.slackHookURL, payload, map[string]string{"Content-Type": "application/json"})
	if code == http.StatusOK {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to send notification to Slack %d: %w", code, err)
	}
	if code >= 300 {
		return fmt.Errorf("received non-2xx response from Slack: %d", code)
	}
	return nil
}

func getHeader(message string) slackBlock {
	return slackBlock{
		Type: "header",
		Text: &item{
			Type: "plain_text",
			Text: message,
		},
	}
}

func (s *Service) getContextWithExtra(extra ...string) slackBlock {
	res := slackBlock{
		Type: "context",
		Elements: []element{
			{
				Type: "mrkdwn",
				Text: fmt.Sprintf("box name: *%s*", s.boxName),
			},
		},
	}
	if len(extra) > 0 {
		for i := range extra {
			res.Elements = append(res.Elements, element{
				Type: "mrkdwn",
				Text: extra[i],
			})
		}
	}
	return res
}

func (s *Service) getContext() slackBlock {
	res := slackBlock{
		Type: "context",
		Elements: []element{
			{
				Type: "mrkdwn",
				Text: fmt.Sprintf("box name: *%s*", s.boxName),
			},
		},
	}
	return res
}

func slackField(title, value string) map[string]any {
	if value == "" {
		value = "-"
	}
	return map[string]any{
		"type": "mrkdwn",
		"text": fmt.Sprintf("*%s*:\t%s", title, value),
	}
}

func slackSafeBody(b []byte, max int) string {
	if len(b) == 0 {
		return "<empty>"
	}
	if len(b) > max {
		b = append(b[:max], []byte("…<truncated>")...)
	}
	s := strings.ReplaceAll(string(b), "```", "`\u200b``")
	return s
}
