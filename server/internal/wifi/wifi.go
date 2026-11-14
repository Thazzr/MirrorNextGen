// Package wifi builds the guest Wi-Fi QR code of the dashboard.
package wifi

import (
	"fmt"
	"strings"

	"rsc.io/qr"
)

// Security values of a network as the QR payload names them.
const (
	WPA    = "WPA"
	NoPass = "nopass"
)

// escaper escapes the characters that are special in the WIFI: payload.
var escaper = strings.NewReplacer(`\`, `\\`, `;`, `\;`, `,`, `\,`, `:`, `\:`, `"`, `\"`)

// Payload returns the text the QR code carries, as phones read it. An open
// network (NoPass) carries no password.
func Payload(ssid, security, password string) string {
	if security == NoPass {
		return "WIFI:T:nopass;S:" + escaper.Replace(ssid) + ";;"
	}
	return "WIFI:T:" + WPA + ";S:" + escaper.Replace(ssid) + ";P:" + escaper.Replace(password) + ";;"
}

// Matrix is a QR code as dark/light modules without the quiet zone.
type Matrix struct {
	Size int
	// Rows has Size strings of '1' (dark) and '0' (light).
	Rows []string
}

// Encode renders text as a QR code at error correction level Q.
func Encode(text string) (Matrix, error) {
	code, err := qr.Encode(text, qr.Q)
	if err != nil {
		return Matrix{}, fmt.Errorf("encode QR code: %w", err)
	}
	rows := make([]string, code.Size)
	var sb strings.Builder
	for y := 0; y < code.Size; y++ {
		sb.Reset()
		for x := 0; x < code.Size; x++ {
			if code.Black(x, y) {
				sb.WriteByte('1')
			} else {
				sb.WriteByte('0')
			}
		}
		rows[y] = sb.String()
	}
	return Matrix{Size: code.Size, Rows: rows}, nil
}
