package httpapi

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
)

func fileToDataURL(r io.Reader) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	if len(data) == 0 {
		return "", fmt.Errorf("empty file")
	}
	mimeType := http.DetectContentType(data)
	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(data)), nil
}

func fileToDataURLFromBytes(data []byte) string {
	mimeType := http.DetectContentType(data)
	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(data))
}

func readAllFromReader(r io.Reader) ([]byte, error) {
	if r == nil {
		return nil, fmt.Errorf("nil reader")
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
