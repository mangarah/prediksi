package notify

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type Telegram struct {
	Token  string
	ChatID string
}

func NewFromEnv() *Telegram {
	return &Telegram{Token: os.Getenv("TELEGRAM_BOT_TOKEN"), ChatID: os.Getenv("TELEGRAM_CHAT_ID")}
}

func (t *Telegram) Enabled() bool {
	return t != nil && t.Token != "" && t.ChatID != ""
}

func (t *Telegram) Send(ctx context.Context, text string) error {
	if !t.Enabled() {
		return nil
	}
	form := url.Values{"chat_id": {t.ChatID}, "text": {text}, "parse_mode": {"HTML"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.telegram.org/bot"+t.Token+"/sendMessage", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: 8 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return fmt.Errorf("telegram: status %d", res.StatusCode)
	}
	return nil
}
