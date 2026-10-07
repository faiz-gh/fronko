package mail

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
	"time"
	"unicode"
)

var codeHTML = template.Must(template.New("code").Parse(`<!doctype html>
<html>
<body style="margin:0;padding:24px;background:#f5f5f4;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;color:#1c1917">
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0">
    <tr><td align="center">
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:440px;background:#ffffff;border-radius:12px;padding:32px">
        <tr><td style="font-size:18px;font-weight:600;padding-bottom:16px">Fronko</td></tr>
        <tr><td style="font-size:15px;line-height:1.5;padding-bottom:20px">{{.Intro}}</td></tr>
        {{- if .Details}}
        <tr><td style="padding-bottom:20px">
          <table role="presentation" cellpadding="0" cellspacing="0" style="width:100%;background:#f5f5f4;border-radius:8px;padding:12px 16px;font-size:14px;line-height:1.8">
            {{- range .Details}}
            <tr><td style="color:#78716c;padding-right:16px;white-space:nowrap">{{.Label}}</td><td style="font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-weight:600;word-break:break-all">{{.Value}}</td></tr>
            {{- end}}
          </table>
        </td></tr>
        {{- end}}
        {{- range .Quotes}}
        <tr><td style="padding-bottom:20px">
          {{- if .Label}}<div style="font-size:13px;color:#78716c;padding-bottom:6px">{{.Label}}</div>{{end}}
          <div style="border-left:3px solid #d6d3d1;padding:4px 0 4px 12px;font-size:14px;line-height:1.6;white-space:pre-wrap;word-break:break-word">{{.Text}}</div>
        </td></tr>
        {{- end}}
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
	Intro string
	// Details are labelled values shown in a box under the intro (e.g. sign-in details).
	Details []detail
	// Quotes are blocks of free text, such as a feedback message, shown as written.
	Quotes  []quote
	Code    string
	Minutes int
	Footer  string
}

type detail struct {
	Label, Value string
}

type quote struct {
	Label, Text string
}

func codeMessage(subject string, data codeEmail) Message {
	subject = cleanSubject(subject)
	var html bytes.Buffer
	if err := codeHTML.Execute(&html, data); err != nil {
		// The template and data are fixed; this can't fail at runtime.
		panic(err)
	}
	intro := data.Intro
	if len(data.Details) > 0 {
		var b strings.Builder
		b.WriteString(intro + "\n")
		for _, d := range data.Details {
			fmt.Fprintf(&b, "\n    %s: %s", d.Label, d.Value)
		}
		intro = b.String()
	}
	for _, q := range data.Quotes {
		var b strings.Builder
		b.WriteString(intro + "\n")
		if q.Label != "" {
			b.WriteString("\n" + q.Label + "\n")
		}
		for _, line := range strings.Split(q.Text, "\n") {
			b.WriteString("\n    " + line)
		}
		intro = b.String()
	}
	text := fmt.Sprintf("%s\n\n%s\n\n— Fronko\n", intro, data.Footer)
	if data.Code != "" {
		text = fmt.Sprintf("%s\n\n    %s\n\nThis code expires in %d minutes. %s\n\n— Fronko\n",
			intro, data.Code, data.Minutes, data.Footer)
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

// MemberInviteMessage tells someone their organisation made them an account,
// with the username and temporary password to sign in with. The password only
// works until they replace it, which they must do on first sign-in.
func MemberInviteMessage(orgName, username, tempPassword string) Message {
	return codeMessage(orgName+" added you to Fronko", codeEmail{
		Intro: orgName + " created a Fronko account for you. Sign in with these details:",
		Details: []detail{
			{"Username", username},
			{"Temporary password", tempPassword},
		},
		Footer: "When you first sign in, you'll confirm this email address with a code we send you, then choose your own password. " +
			"If you weren't expecting this, you can ignore this email.",
	})
}

// cleanSubject turns control characters (line breaks above all) into spaces,
// since subjects can include names and text that users typed.
func cleanSubject(s string) string {
	return strings.Join(strings.FieldsFunc(s, unicode.IsControl), " ")
}

// truncate shortens s to at most n runes, adding an ellipsis when it cuts.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// FeedbackDetails describes feedback for the notice sent to the platform admin.
type FeedbackDetails struct {
	SenderEmail string
	OrgName     string
	Category    string
	Rating      *int16
	PagePath    string
	Message     string
}

// FeedbackNotice tells the platform admin that someone sent feedback.
func FeedbackNotice(f FeedbackDetails) Message {
	details := []detail{
		{"From", f.SenderEmail},
		{"Organisation", f.OrgName},
		{"Category", f.Category},
	}
	if f.Rating != nil {
		details = append(details, detail{"Rating", fmt.Sprintf("%d / 5", *f.Rating)})
	}
	if f.PagePath != "" {
		details = append(details, detail{"Page", f.PagePath})
	}
	return codeMessage("New Fronko feedback ("+f.Category+") from "+f.OrgName, codeEmail{
		Intro:   "Someone sent feedback about Fronko:",
		Details: details,
		Quotes:  []quote{{"", f.Message}},
		Footer:  "Reply to it from the Feedback page of the admin panel.",
	})
}

// FeedbackReplyMessage carries a platform admin's reply to the person who sent feedback.
func FeedbackReplyMessage(original, reply string) Message {
	return codeMessage("Re: your Fronko feedback", codeEmail{
		Intro: "Thanks for your feedback on Fronko. Here's our reply:",
		Quotes: []quote{
			{"", reply},
			{"You wrote:", truncate(original, 1000)},
		},
		Footer: "You can reply to this email to continue the conversation.",
	})
}

// OrgSuspendedMessage tells an organisation's owner that it was suspended, and why.
func OrgSuspendedMessage(orgName, reason string) Message {
	return codeMessage(orgName+" has been suspended on Fronko", codeEmail{
		Intro: "Your organisation " + orgName + " has been suspended. Nobody in it can sign in, " +
			"and its public cards are unavailable until it is reinstated.",
		Quotes: []quote{{"Reason:", reason}},
		Footer: "If you think this is a mistake, reply to this email or contact whoever runs this Fronko server.",
	})
}

// OrgReinstatedMessage tells an organisation's owner that its suspension was lifted.
func OrgReinstatedMessage(orgName string) Message {
	return codeMessage(orgName+" has been reinstated on Fronko", codeEmail{
		Intro:  "Your organisation " + orgName + " has been reinstated. Everyone can sign in again, and its public cards are back online.",
		Footer: "Thanks for your patience.",
	})
}
