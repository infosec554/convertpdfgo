package mailer

import (
	"bytes"
	"html/template"
)

// otpTmpl is shared by OTP-bearing flows: registration, OTP login, password
// reset. Keeping the markup in one place prevents per-handler drift.
var otpTmpl = template.Must(template.New("otp").Parse(`
<html>
  <head>
    <style>
      .container { max-width: 500px; margin: auto; padding: 20px;
        border: 1px solid #eee; border-radius: 10px;
        font-family: Arial, sans-serif; background-color: #f9f9f9; }
      .otp-code { font-size: 28px; font-weight: bold; letter-spacing: 4px;
        color: #2c3e50; margin: 20px 0; }
      .footer { font-size: 12px; color: #888; margin-top: 30px; }
    </style>
  </head>
  <body>
    <div class="container">
      <h2>{{.Greeting}}</h2>
      <p>{{.Lead}}</p>
      <div class="otp-code">{{.Code}}</div>
      <p>{{.Note}}</p>
      <div class="footer">{{.Footer}}</div>
    </div>
  </body>
</html>
`))

// OTPEmailData fills the OTP template.
type OTPEmailData struct {
	Greeting string
	Lead     string
	Code     string
	Note     string
	Footer   string
}

// RenderOTPEmail produces the HTML body for an OTP message. Defaults to
// Uzbek copy if any text field is empty.
func RenderOTPEmail(d OTPEmailData) (string, error) {
	if d.Greeting == "" {
		d.Greeting = "Assalomu alaykum!"
	}
	if d.Lead == "" {
		d.Lead = "Sizning bir martalik kodingiz:"
	}
	if d.Note == "" {
		d.Note = "Iltimos, bu kodni 5 daqiqa ichida kiriting."
	}
	if d.Footer == "" {
		d.Footer = "Agar siz bu so'rovni yubormagan bo'lsangiz, xabarni e'tiborsiz qoldiring."
	}
	var buf bytes.Buffer
	if err := otpTmpl.Execute(&buf, d); err != nil {
		return "", err
	}
	return buf.String(), nil
}
