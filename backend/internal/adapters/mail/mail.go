// Package mail envoie les emails de CodeTroc : par SMTP en production, dans
// le journal en développement.
package mail

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/gitb2pic/codetroc/backend/internal/app"
)

// SMTP envoie par un relais SMTP authentifié, toujours chiffré : TLS direct
// sur le port 465, STARTTLS obligatoire sinon.
type SMTP struct {
	Host     string
	Port     int
	User     string
	Password string
	From     string // ex. « CodeTroc <no-reply@codetroc.labtools.tech> »
	Timeout  time.Duration
}

var _ app.Mailer = SMTP{}

var errHeader = errors.New("mail : saut de ligne interdit dans un en-tête")

// Build construit le message MIME (texte UTF-8, quoted-printable).
func Build(from string, m app.Email, now time.Time) ([]byte, error) {
	for _, h := range []string{from, m.To, m.Subject} {
		if strings.ContainsAny(h, "\r\n") {
			return nil, errHeader
		}
	}
	to, err := mail.ParseAddress(m.To)
	if err != nil {
		return nil, fmt.Errorf("mail : destinataire invalide : %w", err)
	}
	sender, err := mail.ParseAddress(from)
	if err != nil {
		return nil, fmt.Errorf("mail : expéditeur invalide : %w", err)
	}
	id := make([]byte, 12)
	_, _ = rand.Read(id)
	domain := sender.Address[strings.LastIndex(sender.Address, "@")+1:]
	var b bytes.Buffer
	fmt.Fprintf(&b, "From: %s\r\n", sender.String())
	fmt.Fprintf(&b, "To: %s\r\n", to.String())
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", m.Subject))
	fmt.Fprintf(&b, "Date: %s\r\n", now.Format(time.RFC1123Z))
	fmt.Fprintf(&b, "Message-ID: <%s@%s>\r\n", hex.EncodeToString(id), domain)
	b.WriteString("MIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n")
	qp := quotedprintable.NewWriter(&b)
	if _, err := qp.Write([]byte(strings.ReplaceAll(m.Text, "\n", "\r\n"))); err != nil {
		return nil, err
	}
	if err := qp.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// Send envoie un email ; la connexion est fermée à la fin de l'envoi.
func (s SMTP) Send(ctx context.Context, m app.Email) error {
	msg, err := Build(s.From, m, time.Now())
	if err != nil {
		return err
	}
	timeout := s.Timeout
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	addr := net.JoinHostPort(s.Host, strconv.Itoa(s.Port))
	dialer := &net.Dialer{Timeout: timeout}
	tlsCfg := &tls.Config{ServerName: s.Host, MinVersion: tls.VersionTLS12}
	var conn net.Conn
	if s.Port == 465 {
		conn, err = (&tls.Dialer{NetDialer: dialer, Config: tlsCfg}).DialContext(ctx, "tcp", addr)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("mail : connexion à %s : %w", addr, err)
	}
	_ = conn.SetDeadline(time.Now().Add(timeout))
	c, err := smtp.NewClient(conn, s.Host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("mail : %w", err)
	}
	defer func() { _ = c.Close() }()
	if s.Port != 465 {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return errors.New("mail : le serveur ne propose pas STARTTLS")
		}
		if err := c.StartTLS(tlsCfg); err != nil {
			return fmt.Errorf("mail : STARTTLS : %w", err)
		}
	}
	if s.User != "" {
		if err := c.Auth(smtp.PlainAuth("", s.User, s.Password, s.Host)); err != nil {
			return fmt.Errorf("mail : authentification : %w", err)
		}
	}
	from, _ := mail.ParseAddress(s.From)
	to, _ := mail.ParseAddress(m.To)
	if err := c.Mail(from.Address); err != nil {
		return fmt.Errorf("mail : MAIL FROM : %w", err)
	}
	if err := c.Rcpt(to.Address); err != nil {
		return fmt.Errorf("mail : RCPT TO : %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("mail : DATA : %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("mail : envoi : %w", err)
	}
	return c.Quit()
}

// Log écrit les emails dans le journal au lieu de les envoyer (développement
// seulement : les liens contiennent des jetons).
type Log struct{ Logger *slog.Logger }

// Send journalise l'email.
func (l Log) Send(_ context.Context, m app.Email) error {
	l.Logger.Info("email (non envoyé, développement)", slog.String("a", m.To), slog.String("sujet", m.Subject), slog.String("texte", m.Text))
	return nil
}
