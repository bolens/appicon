package nounproject

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// signRequest adds an OAuth 1.0a Authorization header (HMAC-SHA1) for Noun Project.
func signRequest(req *http.Request, consumerKey, consumerSecret string) error {
	nonce, err := randomNonce(16)
	if err != nil {
		return err
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	oauth := map[string]string{
		"oauth_consumer_key":     consumerKey,
		"oauth_nonce":            nonce,
		"oauth_signature_method": "HMAC-SHA1",
		"oauth_timestamp":        timestamp,
		"oauth_version":          "1.0",
	}
	baseURL := req.URL.Scheme + "://" + req.URL.Host + req.URL.EscapedPath()
	params := url.Values{}
	for k, v := range oauth {
		params.Set(k, v)
	}
	for k, vs := range req.URL.Query() {
		for _, v := range vs {
			params.Add(k, v)
		}
	}
	baseString := strings.ToUpper(req.Method) + "&" + percentEncode(baseURL) + "&" + percentEncode(normalizeParams(params))
	key := percentEncode(consumerSecret) + "&"
	mac := hmac.New(sha1.New, []byte(key))
	_, _ = mac.Write([]byte(baseString))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	oauth["oauth_signature"] = sig

	keys := make([]string, 0, len(oauth))
	for k := range oauth {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf(`%s="%s"`, k, percentEncode(oauth[k])))
	}
	req.Header.Set("Authorization", "OAuth "+strings.Join(parts, ", "))
	return nil
}

func normalizeParams(v url.Values) string {
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	first := true
	for _, k := range keys {
		vals := append([]string(nil), v[k]...)
		sort.Strings(vals)
		for _, val := range vals {
			if !first {
				b.WriteByte('&')
			}
			first = false
			b.WriteString(percentEncode(k))
			b.WriteByte('=')
			b.WriteString(percentEncode(val))
		}
	}
	return b.String()
}

func percentEncode(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '.' || c == '_' || c == '~' {
			b.WriteByte(c)
			continue
		}
		fmt.Fprintf(&b, "%%%02X", c)
	}
	return b.String()
}

func randomNonce(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
