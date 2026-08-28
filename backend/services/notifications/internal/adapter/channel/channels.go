// Package channel implements the Channel SPI adapters: in-app inbox,
// email (net/smtp), SMS (Kavenegar HTTP), and generic signed webhook.
// Dev-safe: every provider has a log adapter selected when its env
// config is absent, so the pipeline runs without external credentials.
package channel

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"net/smtp"
	"net/url"
	"strings"
	"time"

	"unital/backend/pkg/config"
	"unital/backend/services/notifications/internal/domain"
)

// Message and Channel live in domain; aliases keep adapter call sites short.
type Message = domain.Message
type Channel = domain.Channel

// ---------- in-app ----------

// InApp writes to the user's inbox store.
type InApp struct{ Inbox domain.InboxStore }

func (c *InApp) Name() string { return domain.ChanInApp }

func (c *InApp) Send(ctx context.Context, m Message) (string, error) {
	msg := &domain.InboxMessage{
		ID: "inbox-" + m.Meta["delivery_id"], UserID: m.To, Title: m.Title,
		Body: m.Body, CampaignID: m.Meta["campaign_id"], CreatedAt: time.Now().UTC(),
	}
	if err := c.Inbox.Push(ctx, msg); err != nil {
		return "", err
	}
	return msg.ID, nil
}

// ---------- log (dev default for external providers) ----------

type LogChannel struct{ name string }

func NewLog(name string) *LogChannel { return &LogChannel{name: name} }

func (c *LogChannel) Name() string { return c.name }

func (c *LogChannel) Send(_ context.Context, m Message) (string, error) {
	slog.Info("channel.send", "channel", c.name, "to", m.To, "title", m.Title, "body", m.Body)
	return "log-" + c.name, nil
}

// ---------- email (net/smtp) ----------

// SMTPSender abstracts smtp.SendMail for tests.
type SMTPSender func(addr string, auth smtp.Auth, from string, to []string, msg []byte) error

// Email sends over SMTP; falls back to LogChannel when unconfigured.
func Email() Channel {
	host := config.Str("SMTP_HOST", "")
	if host == "" {
		return NewLog(domain.ChanEmail)
	}
	port := config.Str("SMTP_PORT", "587")
	user := config.Str("SMTP_USER", "")
	pass := config.Str("SMTP_PASS", "")
	from := config.Str("SMTP_FROM", "no-reply@unital.app")
	addr := host + ":" + port
	var auth smtp.Auth
	if user != "" {
		auth = smtp.PlainAuth("", user, pass, host)
	}
	return &smtpChannel{addr: addr, auth: auth, from: from, send: smtp.SendMail}
}

type smtpChannel struct {
	addr string
	auth smtp.Auth
	from string
	send SMTPSender
}

func (c *smtpChannel) Name() string { return domain.ChanEmail }

func (c *smtpChannel) Send(_ context.Context, m Message) (string, error) {
	body := strings.ReplaceAll(m.Body, "\n", "\r\n")
	msg := "From: " + c.from + "\r\n" +
		"To: " + m.To + "\r\n" +
		"Subject: " + m.Title + "\r\n" +
		"MIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n" +
		body
	if err := c.send(c.addr, c.auth, c.from, []string{m.To}, []byte(msg)); err != nil {
		return "", fmt.Errorf("smtp send: %w", err)
	}
	return "smtp:" + m.To, nil
}

// ---------- SMS (Kavenegar; Ghasedak/MelliPayamak as siblings later) ----------

// SMS selects the configured provider; falls back to LogChannel.
func SMS() Channel {
	provider := config.Str("SMS_PROVIDER", "log")
	switch provider {
	case "kavenegar":
		key := config.Str("KAVENEGAR_API_KEY", "")
		if key == "" {
			return NewLog(domain.ChanSMS)
		}
		sender := config.Str("SMS_SENDER", "")
		return &kavenegar{apiKey: key, sender: sender, client: &http.Client{Timeout: 10 * time.Second}}
	default:
		return NewLog(domain.ChanSMS)
	}
}

type kavenegar struct {
	apiKey string
	sender string
	client *http.Client
}

func (c *kavenegar) Name() string { return domain.ChanSMS }

func (c *kavenegar) Send(_ context.Context, m Message) (string, error) {
	endpoint := fmt.Sprintf("https://api.kavenegar.com/v1/%s/sms/send.json", c.apiKey)
	form := url.Values{}
	form.Set("receptor", strings.TrimPrefix(m.To, "+"))
	form.Set("message", m.Title+"\n"+m.Body)
	if c.sender != "" {
		form.Set("sender", c.sender)
	}
	resp, err := c.client.PostForm(endpoint, form)
	if err != nil {
		return "", fmt.Errorf("kavenegar post: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("kavenegar status %d", resp.StatusCode)
	}
	return fmt.Sprintf("kavenegar:%s:%s", m.To, time.Now().Format("150405")), nil
}

// ---------- webhook (signed) ----------

// Webhook posts JSON to WEBHOOK_URL with an HMAC-SHA256 signature header.
func Webhook() Channel {
	target := config.Str("WEBHOOK_URL", "")
	secret := config.Str("WEBHOOK_SECRET", "")
	if target == "" {
		return NewLog(domain.ChanWebhook)
	}
	return &webhookChannel{url: target, secret: secret, client: &http.Client{Timeout: 10 * time.Second}}
}

type webhookChannel struct {
	url    string
	secret string
	client *http.Client
}

func (c *webhookChannel) Name() string { return domain.ChanWebhook }

func (c *webhookChannel) Send(_ context.Context, m Message) (string, error) {
	payload := fmt.Sprintf(`{"to":%q,"title":%q,"body":%q,"meta":%q}`,
		m.To, m.Title, m.Body, m.Meta["campaign_id"])
	req, err := http.NewRequest(http.MethodPost, c.url, strings.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.secret != "" {
		mac := hmac.New(sha256.New, []byte(c.secret))
		mac.Write([]byte(payload))
		req.Header.Set("X-Unital-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("webhook post: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("webhook status %d", resp.StatusCode)
	}
	return "webhook:" + c.url, nil
}
