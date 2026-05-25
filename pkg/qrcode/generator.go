package qrcode

import (
	"encoding/base64"
	"fmt"

	qr "github.com/skip2/go-qrcode"
)

func GenerateBase64(url string) (string, error) {
	pngData, err := qr.Encode(url, qr.Medium, 256)
	if err != nil {
		return "", fmt.Errorf("failed to generate qrcode: %w", err)
	}

	base64Str := base64.StdEncoding.EncodeToString(pngData)
	return "data:image/png;base64," + base64Str, nil
}
