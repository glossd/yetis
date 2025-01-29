package common

import (
	"bytes"
	"testing"
)

func TestYetisConfig(t *testing.T) {
	in := `
logdir: /tmp # yetis.log will be stored in there. Defaults to /tmp
alerting:
  mail: # add SMPT creds of your smpt server for alerting
    host: smtp.host.com
    port: 587
    from: noreply@mail.com
    to:
      - yourmail@mail.com
    username: authUser
    password: authPass
`

	res := readServerConfig(bytes.NewBufferString(in))
	if res.Logdir != "/tmp" || res.Alerting.Mail.Host != "smtp.host.com" || len(res.Alerting.Mail.To) != 1 {
		t.Fatal("Wrong config:", res)
	}
	assert(t, res.Alerting.Mail.Validate(), nil)
}

func TestSendEmail(t *testing.T) {
	t.SkipNow()

	c := ReadServerConfig("../tmp/yetis.yml")
	assert(t, c.Alerting.Mail.Validate(), nil)
	err := c.Alerting.Mail.Send("Hello Test", "This came from TestSendEmail")
	assert(t, err, nil)
}
