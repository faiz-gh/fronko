// Package mail sends the app's transactional email (verification and
// password-reset codes) over SMTP.
package mail

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// Message is one email, with a plain-text and an HTML body.
type Message struct {
	Subject string
	Text    string
	HTML    string
	// ReplyTo, when set, is where replies to the email go instead of the sender.
	ReplyTo string
}

type Sender interface {
	Send(ctx context.Context, to string, m Message) error
}

// SMTPConfig describes the outgoing mail server. Port 465 uses implicit TLS;
// any other port upgrades with STARTTLS when the server offers it.
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string // e.g. "Fronko <no-reply@fronko.app>"
}

type SMTPSender struct {
	cfg  SMTPConfig
	from *mail.Address
}

func NewSMTPSender(cfg SMTPConfig) (*SMTPSender, error) {
	from, err := mail.ParseAddress(cfg.From)
	if err != nil {
		return nil, fmt.Errorf("invalid from address %q: %w", cfg.From, err)
	}
	return &SMTPSender{cfg: cfg, from: from}, nil
}

func (s *SMTPSender) Send(ctx context.Context, to string, m Message) error {
	raw, err := buildMessage(s.from, to, m, time.Now())
	if err != nil {
		return err
	}

	addr := net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.cfg.Port))
	tlsConfig := &tls.Config{ServerName: s.cfg.Host, MinVersion: tls.VersionTLS12}
	implicitTLS := s.cfg.Port == 465

	var conn net.Conn
	if implicitTLS {
		conn, err = (&tls.Dialer{Config: tlsConfig}).DialContext(ctx, "tcp", addr)
	} else {
		conn, err = (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("connecting to %s: %w", addr, err)
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	c, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		conn.Close()
		return err
	}
	defer c.Close()

	if !implicitTLS {
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(tlsConfig); err != nil {
				return fmt.Errorf("starttls: %w", err)
			}
		}
	}
	// smtp.PlainAuth refuses to send credentials over an unencrypted
	// connection (except to localhost), so a misconfigured server fails loudly.
	if s.cfg.Username != "" {
		if err := c.Auth(smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	if err := c.Mail(s.from.Address); err != nil {
		return err
	}
	if err := c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(raw); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

// LogSender prints messages instead of sending them. It's the fallback when
// no SMTP server is configured, so codes are still reachable in development.
type LogSender struct{}

func (LogSender) Send(_ context.Context, to string, m Message) error {
	log.Printf("mail (SMTP not configured) to=%s subject=%q\n%s", to, m.Subject, m.Text)
	return nil
}

// buildMessage renders a multipart/alternative RFC 5322 message.
func buildMessage(from *mail.Address, to string, m Message, now time.Time) ([]byte, error) {
	// Addresses are validated before they get here, but never let one smuggle in headers.
	if strings.ContainsAny(to, "\r\n") {
		return nil, errors.New("invalid recipient address")
	}
	toAddr, err := mail.ParseAddress(to)
	if err != nil {
		return nil, fmt.Errorf("invalid recipient address: %w", err)
	}
	var replyTo *mail.Address
	if m.ReplyTo != "" {
		if strings.ContainsAny(m.ReplyTo, "\r\n") {
			return nil, errors.New("invalid reply-to address")
		}
		if replyTo, err = mail.ParseAddress(m.ReplyTo); err != nil {
			return nil, fmt.Errorf("invalid reply-to address: %w", err)
		}
	}

	boundary, err := randomHex(16)
	if err != nil {
		return nil, err
	}
	msgID, err := randomHex(16)
	if err != nil {
		return nil, err
	}
	domain := from.Address[strings.LastIndex(from.Address, "@")+1:]

	var b bytes.Buffer
	header := func(k, v string) { fmt.Fprintf(&b, "%s: %s\r\n", k, v) }
	header("From", from.String())
	header("To", toAddr.String())
	if replyTo != nil {
		header("Reply-To", replyTo.String())
	}
	header("Subject", mime.QEncoding.Encode("utf-8", m.Subject))
	header("Date", now.Format(time.RFC1123Z))
	header("Message-ID", fmt.Sprintf("<%s@%s>", msgID, domain))
	header("MIME-Version", "1.0")
	header("Content-Type", fmt.Sprintf("multipart/alternative; boundary=%q", boundary))
	b.WriteString("\r\n")

	for _, part := range []struct{ contentType, body string }{
		{"text/plain; charset=utf-8", m.Text},
		{"text/html; charset=utf-8", m.HTML},
	} {
		fmt.Fprintf(&b, "--%s\r\n", boundary)
		header("Content-Type", part.contentType)
		header("Content-Transfer-Encoding", "quoted-printable")
		b.WriteString("\r\n")
		qp := quotedprintable.NewWriter(&b)
		if _, err := qp.Write([]byte(part.body)); err != nil {
			return nil, err
		}
		if err := qp.Close(); err != nil {
			return nil, err
		}
		b.WriteString("\r\n")
	}
	fmt.Fprintf(&b, "--%s--\r\n", boundary)
	return b.Bytes(), nil
}

func randomHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// SendTimeout bounds one background send, including the SMTP handshake.
const SendTimeout = 30 * time.Second

// SendAsync delivers mail in the background so a slow SMTP server doesn't hold
// up the request, and so responses take the same time whether or not mail
// goes out. Failures are logged.
func SendAsync(s Sender, to string, m Message) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), SendTimeout)
		defer cancel()
		if err := s.Send(ctx, to, m); err != nil {
			log.Printf("sending %q: %v", m.Subject, err)
		}
	}()
}
