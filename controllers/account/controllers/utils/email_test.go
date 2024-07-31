package utils

import "testing"

func TestSMTPConfig_SendEmail(t *testing.T) {
	c := &SMTPConfig{
		ServerHost: "smtp.feishu.cn",
		ServerPort: 465,
		FromEmail:  "noreply@sealos.io",
		Passwd:     "EribJHnuEqGKDn8W",
		EmailTitle: "【 Sealos Cloud 】",
	}
	err := c.SendEmail("当前工作空间所属账户余额不足，系统将为您暂停服务，请及时充值，以免影响您的正常使用。", "1748756566@qq.com")
	if err != nil {
		t.Errorf("SendEmail() error = %v", err)
	}
	t.Logf("send email success!")
}
