package slack

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// KeyFromEnv: SLACK_ENC_KEY (base64, 32 bytes) if set, else a key derived from JWT_SECRET_KEY.
func KeyFromEnv() []byte {
	if k, err := base64.StdEncoding.DecodeString(os.Getenv("SLACK_ENC_KEY")); err == nil && len(k) == 32 {
		return k
	}
	if os.Getenv("SLACK_ENC_KEY") != "" {
		log.Println("WARNING: SLACK_ENC_KEY is not a base64 32-byte key; falling back to a key derived from JWT_SECRET_KEY")
	}
	// Note: rotating JWT_SECRET_KEY changes this key, and the stored Slack token must then be re-entered.
	h := sha256.Sum256([]byte("baro-slack:" + os.Getenv("JWT_SECRET_KEY")))
	return h[:]
}

// Seal encrypts a token with AES-256-GCM; the result is base64(nonce|ciphertext).
func Seal(key []byte, plain string) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(gcm.Seal(nonce, nonce, []byte(plain), nil)), nil
}

func Open(key []byte, sealed string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(sealed)
	if err != nil {
		return "", errors.New("slack: stored token is unreadable")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil || len(raw) < gcm.NonceSize() {
		return "", errors.New("slack: stored token is unreadable")
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return "", errors.New("slack: stored token is unreadable")
	}
	return string(plain), nil
}

func minutes(hhmm string, def int) int {
	parts := strings.Split(hhmm, ":")
	if len(parts) != 2 {
		return def
	}
	h, e1 := strconv.Atoi(parts[0])
	m, e2 := strconv.Atoi(parts[1])
	if e1 != nil || e2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return def
	}
	return h*60 + m
}

// InQuiet reports whether now (Bangkok) falls inside the learner-DM quiet window.
// Empty or invalid times use the default 21:00–08:00. The window may wrap midnight.
func InQuiet(now time.Time, from, to string) bool {
	bkk := time.FixedZone("BKK", 7*3600)
	n := now.In(bkk)
	cur := n.Hour()*60 + n.Minute()
	f, t := minutes(from, 21*60), minutes(to, 8*60)
	if f == t {
		return false
	}
	if f < t {
		return cur >= f && cur < t
	}
	return cur >= f || cur < t
}
