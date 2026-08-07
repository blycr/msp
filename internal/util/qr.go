package util

import (
	"strings"

	qrcode "github.com/skip2/go-qrcode"
)

// Terminal ANSI block styles: every QR module is rendered as a 2-column wide
// space with a solid background, so contrast does not depend on the terminal's
// foreground color. 深色块 = 黑背景（扫码识别的深色模块），浅色块 = 白背景。
const (
	qrDarkModule  = "\x1b[40m  \x1b[0m"
	qrLightModule = "\x1b[47m  \x1b[0m"
)

// QRCodeTerminal renders content (typically a LAN URL) as an ANSI-colored QR
// code that can be scanned with a phone camera from the terminal output.
// The returned string contains the quiet zone provided by the QR bitmap.
func QRCodeTerminal(content string) (string, error) {
	q, err := qrcode.New(content, qrcode.Medium)
	if err != nil {
		return "", err
	}

	bmp := q.Bitmap() // [][]bool, true = dark module, includes quiet zone
	var sb strings.Builder
	for _, row := range bmp {
		for _, dark := range row {
			if dark {
				sb.WriteString(qrDarkModule)
			} else {
				sb.WriteString(qrLightModule)
			}
		}
		sb.WriteByte('\n')
	}
	return sb.String(), nil
}
