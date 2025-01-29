package common

import (
	"fmt"
	"io"
	"log"
	"net/smtp"
	"os"
	"reflect"
	yaml "sigs.k8s.io/yaml/goyaml.v2"
)

type YetisConfig struct {
	Logdir   string
	Alerting Alerting
}

func (yc YetisConfig) WithDefaults() YetisConfig {
	if yc.Logdir == "" {
		yc.Logdir = "/tmp"
	}
	return yc
}

type Alerting struct {
	Mail Mail
}

func (a Alerting) Send(title, description string) error {
	var errStr string
	if a.Mail.Validate() == nil {
		err := a.Mail.Send(title, description)
		if err != nil {
			errStr += err.Error() + " + "
		}
	}
	if errStr != "" {
		return fmt.Errorf("alert failure: %s", errStr)
	}
	return nil
}

type Mail struct {
	Host     string
	Port     int
	From     string
	To       []string
	Username string
	Password string
}

func (m Mail) Validate() error {
	if m.Host == "" {
		return fmt.Errorf("mail: host can't be empty")
	}
	if m.From == "" {
		return fmt.Errorf("mail: from field can't be empty")
	}
	if len(m.To) == 0 {
		return fmt.Errorf("mail: to field can't be empty")
	}
	return nil
}

func (m Mail) Send(title, description string) error {
	smtpAuth := smtp.PlainAuth("", m.Username, m.Password, m.Host)
	address := fmt.Sprintf("%s:%d", m.Host, m.Port)
	msg := fmt.Sprintf("From: %s\nSubject: %s\n\n%s", m.From, title, description)
	return smtp.SendMail(address, smtpAuth, m.From, m.To, []byte(msg))
}

func ReadServerConfig(path string) YetisConfig {
	f, err := os.Open(path)
	if err != nil {
		log.Fatalf("Couldn't open server config: %s", err)
	}
	return readServerConfig(f).WithDefaults()
}

func readServerConfig(f io.Reader) YetisConfig {
	str, err := io.ReadAll(f)
	if err != nil {
		log.Fatalf("Couldn't read server config: %s", err)
	}
	var c YetisConfig
	err = yaml.Unmarshal(str, &c)
	if err != nil {
		log.Fatalf("Failed to unmarshal config: %s", err)
	}
	if !reflect.ValueOf(c.Alerting.Mail).IsZero() {
		err = c.Alerting.Mail.Validate()
		if err != nil {
			log.Fatalf("Mail validation failed: %s", err)
		}
	}
	return c
}
