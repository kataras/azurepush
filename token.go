package azurepush

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"sync"
	"time"
)

// GenerateSASToken creates a Shared Access Signature (SAS) token for Azure Notification Hub.
func GenerateSASToken(resourceUri, keyName, key string, duration time.Duration) (string, error) {
	targetUri := url.QueryEscape(resourceUri)
	expiry := time.Now().Add(duration).Unix()
	signature := fmt.Sprintf("%s\n%d", resourceUri, expiry)
	h := hmac.New(sha256.New, []byte(key))
	h.Write([]byte(signature))
	sig := url.QueryEscape(base64.StdEncoding.EncodeToString(h.Sum(nil)))
	token := fmt.Sprintf("SharedAccessSignature sr=%s&sig=%s&se=%d&skn=%s", targetUri, sig, expiry, keyName)
	return token, nil
}

// TokenManager manages the lifecycle of SAS tokens.
type TokenManager struct {
	cfg       Configuration
	token     string
	expiresAt time.Time
	mutex     sync.Mutex
}

// NewTokenManager creates a new TokenManager.
func NewTokenManager(cfg Configuration) *TokenManager {
	return &TokenManager{cfg: cfg}
}

// GetToken returns a valid SAS token, refreshing it if necessary.
func (tm *TokenManager) GetToken() (string, error) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	if tm.token == "" || time.Now().After(tm.expiresAt.Add(-5*time.Minute)) {
		resourceURI := "https://" + tm.cfg.Namespace + ".servicebus.windows.net/" + tm.cfg.HubName
		token, err := GenerateSASToken(resourceURI, tm.cfg.KeyName, tm.cfg.KeyValue, tm.cfg.TokenValidity)
		if err != nil {
			return "", err
		}
		tm.token = token
		tm.expiresAt = time.Now().Add(tm.cfg.TokenValidity)
	}
	return tm.token, nil
}
