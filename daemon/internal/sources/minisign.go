package sources

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"strings"
)

// verifyMinisign checks sig, the text of a minisign signature file, over
// msg against keys, each a minisign public key file or its base64 line.
// Only the "Ed" algorithm (Ed25519 over the message itself) is accepted:
// it is what the coursive-plugins publish workflow writes. The trusted
// comment is covered by the global signature, so it is checked too.
func verifyMinisign(msg, sig []byte, keys []string) error {
	lines := strings.Split(strings.ReplaceAll(string(sig), "\r\n", "\n"), "\n")
	if len(lines) < 4 || !strings.HasPrefix(lines[0], "untrusted comment: ") || !strings.HasPrefix(lines[2], "trusted comment: ") {
		return errors.New("签名格式无效")
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(lines[1]))
	if err != nil || len(raw) != 2+8+ed25519.SignatureSize || string(raw[:2]) != "Ed" {
		return errors.New("签名格式无效")
	}
	global, err := base64.StdEncoding.DecodeString(strings.TrimSpace(lines[3]))
	if err != nil || len(global) != ed25519.SignatureSize {
		return errors.New("签名格式无效")
	}
	keyID, signature := raw[2:10], raw[10:]
	trusted := strings.TrimPrefix(lines[2], "trusted comment: ")
	for _, k := range keys {
		pk, ok := minisignKey(k)
		if !ok || !bytes.Equal(pk[2:10], keyID) {
			continue
		}
		pub := ed25519.PublicKey(pk[10:])
		if !ed25519.Verify(pub, msg, signature) || !ed25519.Verify(pub, append(append([]byte{}, signature...), trusted...), global) {
			return errors.New("签名与内容不符")
		}
		return nil
	}
	return errors.New("签名不是由受信任的密钥生成的")
}

// minisignKey decodes a public key: "Ed", the 8-byte key id and the key.
func minisignKey(text string) ([]byte, bool) {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(lines[len(lines)-1]))
	if err != nil || len(b) != 2+8+ed25519.PublicKeySize || string(b[:2]) != "Ed" {
		return nil, false
	}
	return b, true
}
