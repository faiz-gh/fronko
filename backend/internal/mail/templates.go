package mail

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
	"time"
)

var codeHTML = template.Must(template.New("code").Parse(`<!doctype html>
<html>
<body style="margin:0;padding:24px;background:#f5f5f4;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;color:#1c1917">
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0">
    <tr><td align="center">
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:440px;background:#ffffff;border-radius:12px;padding:32px">
        <tr><td style="font-size:18px;font-weight:600;padding-bottom:16px">Fronko</td></tr>
        <tr><td style="font-size:15px;line-height:1.5;padding-bottom:20px">{{.Intro}}</td></tr>
        {{- if .Code}}
        <tr><td style="font-size:32px;font-weight:600;letter-spacing:8px;font-family:ui-monospace,SFMono-Regular,Menlo,monospace;padding-bottom:20px">{{.Code}}</td></tr>
        <tr><td style="font-size:13px;line-height:1.5;color:#78716c">This code expires in {{.Minutes}} minutes. {{.Footer}}</td></tr>
        {{- else}}
        <tr><td style="font-size:13px;line-height:1.5;color:#78716c">{{.Footer}}</td></tr>
        {{- end}}
      </table>
    </td></tr>
  </table>
</body>
</html>`))

type codeEmail struct {
	Intro   string
	Code    string
	Minutes int
	Footer  string
}

func codeMessage(subject string, data codeEmail) Message {
	var html bytes.Buffer
	if err := codeHTML.Execute(&html, data); err != nil {
		// The template and data are fixed; this can't fail at runtime.
		panic(err)
	}
	text := fmt.Sprintf("%s\n\n%s\n\n— Fronko\n", data.Intro, data.Footer)
	if data.Code != "" {
		text = fmt.Sprintf("%s\n\n    %s\n\nThis code expires in %d minutes. %s\n\n— Fronko\n",
			data.Intro, data.Code, data.Minutes, data.Footer)
	}
	return Message{Subject: subject, Text: text, HTML: html.String()}
}

// VerifyEmailMessage carries the code that confirms a user owns their address.
func VerifyEmailMessage(code string, ttl time.Duration) Message {
	return codeMessage(code+" is your Fronko verification code", codeEmail{
		Intro:   "Enter this code to verify your email address:",
		Code:    code,
		Minutes: int(ttl.Minutes()),
		Footer:  "If you didn't create a Fronko account, you can ignore this email.",
	})
}

// ResetPasswordMessage carries the code that lets a user set a new password.
func ResetPasswordMessage(code string, ttl time.Duration) Message {
	return codeMessage("Reset your Fronko password", codeEmail{
		Intro:   "Someone asked to reset the password for your Fronko account. Enter this code to choose a new one:",
		Code:    code,
		Minutes: int(ttl.Minutes()),
		Footer:  "If this wasn't you, ignore this email; your password won't change.",
	})
}

// ChangeEmailMessage goes to the new address when a verified account changes its email.
func ChangeEmailMessage(code string, ttl time.Duration) Message {
	return codeMessage(code+" is your code to confirm your new Fronko email", codeEmail{
		Intro:   "Enter this code to make this address the email for your Fronko account:",
		Code:    code,
		Minutes: int(ttl.Minutes()),
		Footer:  "If you didn't ask for this, you can ignore this email; nothing will change.",
	})
}

// EmailChangedNotice tells the previous address that the account moved to newEmail.
func EmailChangedNotice(newEmail string) Message {
	return codeMessage("Your Fronko email was changed", codeEmail{
		Intro:  "The email for your Fronko account was changed to " + MaskEmail(newEmail) + ". This address will no longer receive account emails.",
		Footer: "If this wasn't you, someone else knows your password. Contact whoever runs this Fronko server to get your account back.",
	})
}

// MaskEmail hides most of the local part: "nina@example.com" → "n***@example.com".
func MaskEmail(email string) string {
	at := strings.LastIndex(email, "@")
	if at < 1 {
		return "***"
	}
	return email[:1] + "***" + email[at:]
}
