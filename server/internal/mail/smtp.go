package mail

import (
	"encoding/base64"
	"fmt"
	"net"
	"net/smtp"
	"strconv"
	"strings"
)

type Sender interface {
	Send(to, subject, body string) error
}

type SMTP struct {
	Address  string
	Port     int
	Username string
	Password string
	From     string
}

func (s *SMTP) Send(to, subject, body string) error {
	host := strings.TrimSpace(s.Address)
	if host == "" {
		return fmt.Errorf("SMTP address is required")
	}
	address := host
	if _, _, err := net.SplitHostPort(host); err != nil {
		address = net.JoinHostPort(host, strconv.Itoa(s.Port))
	}
	from := strings.TrimSpace(s.From)
	if from == "" || strings.TrimSpace(to) == "" {
		return fmt.Errorf("mail sender and recipient are required")
	}
	var auth smtp.Auth
	if s.Username != "" {
		auth = smtp.PlainAuth("", s.Username, s.Password, strings.Split(address, ":")[0])
	}
	message := strings.Join([]string{
		"From: " + from,
		"To: " + to,
		"Subject: " + mimeSubject(subject),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Transfer-Encoding: 8bit",
		"",
		body,
	}, "\r\n")
	return smtp.SendMail(address, auth, from, []string{to}, []byte(message))
}

func mimeSubject(subject string) string {
	if isASCII(subject) {
		return subject
	}
	return "=?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte(subject)) + "?="
}

func isASCII(value string) bool {
	for _, character := range value {
		if character > 127 {
			return false
		}
	}
	return true
}
