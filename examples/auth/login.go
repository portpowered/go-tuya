// Package main demonstrates Tuya QR-code authentication.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/portpowered/go-tuya/pkg/tuya"
	"github.com/yeqown/go-qrcode/v2"
	"github.com/yeqown/go-qrcode/writer/terminal"
)

func main() {
	accessCode := os.Getenv("TUYA_ACCESS_CODE")
	if accessCode == "" {
		log.Fatal("set TUYA_ACCESS_CODE before running this example")
	}

	ctx := context.Background()
	base, err := tuya.NewClient()
	if err != nil {
		log.Fatal("could not configure the Tuya client")
	}
	client := base.NewSession(tuya.Tokens{})
	login, err := client.AuthService.GenerateQrCodeForLogin(ctx, tuya.LoginRequest{
		AccessCode: accessCode,
		Schema:     tuya.AuthenticationSchema,
	})
	if err != nil {
		log.Fatal("could not start QR-code login; redact credentials before inspecting diagnostics")
	}

	fmt.Println("Scan the QR code with the Tuya or Smart Life app and approve the login.")
	qr, err := qrcode.New(login.QrFormattedCode)
	if err != nil {
		log.Fatal("could not render the login QR code")
	}
	if err := qr.Save(terminal.New()); err != nil {
		log.Fatal("could not display the login QR code")
	}
	fmt.Println("After the app confirms, press Enter to finish authentication.")
	_, _ = fmt.Scanln()

	tokens, err := client.AuthService.ValidateLoginCode(ctx, tuya.ValidateLoginCodeRequest{
		LoginCode: login.Code,
		UserCode:  accessCode,
	})
	if err != nil {
		log.Fatal("could not validate the login; redact credentials before inspecting diagnostics")
	}
	_ = tokens // Store token values with application-managed secret storage; do not print them.
	fmt.Println("Login succeeded. Persist the returned tokens in secure application storage.")
}
