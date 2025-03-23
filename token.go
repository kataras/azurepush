package azurepush

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/google/uuid"
)

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

// GenerateSASToken creates a Shared Access Signature (SAS) token for Azure Notification Hub.
func GenerateSASToken(resourceUri, keyName, key string, duration time.Duration) (string, error) {
	if resourceUri == "" || keyName == "" || key == "" {
		return "", fmt.Errorf("missing required parameter")
	}

	encodedURI := url.QueryEscape(resourceUri)

	// TTL: 1 week from now
	// ttl := time.Now().Unix() + 60*60*24*7 // seconds

	ttl := time.Now().Add(duration).Unix()
	// Signature: encoded URI + "\n" + expiry timestamp
	signingString := fmt.Sprintf("%s\n%d", encodedURI, ttl)

	h := hmac.New(sha256.New, []byte(key))
	h.Write([]byte(signingString))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))
	encodedSig := url.QueryEscape(signature)

	token := fmt.Sprintf(
		"SharedAccessSignature sr=%s&sig=%s&se=%d&skn=%s",
		encodedURI,
		encodedSig,
		ttl,
		keyName,
	)

	return token, nil
}

// ValidateToken checks if a SAS token is valid.
// Expecting 404 or 200 if token is valid
func ValidateToken(ctx context.Context, httpClient *http.Client, namespace, hubName, token string) error {
	// Dummy installation ID — Azure will return 404 if not found, which is OK
	dummyInstallationID := uuid.NewString()

	url := fmt.Sprintf(
		"https://%s.servicebus.windows.net/%s/installations/%s?api-version=2020-06",
		namespace,
		hubName,
		dummyInstallationID,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create validation request: %w", err)
	}
	req.Header.Set("Authorization", token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send validation request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound {
		return nil
	}

	b, _ := io.ReadAll(resp.Body)
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return fmt.Errorf("unauthorized: SAS token is invalid or expired: %s", string(b))
	default:
		return fmt.Errorf("unexpected status code: %d: %s", resp.StatusCode, string(b))
	}
}
