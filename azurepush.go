package azurepush

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// HTTPClient is the client used for HTTP requests.
// It can be overridden for testing.
var HTTPClient = &http.Client{Timeout: 10 * time.Second}

// NotificationMessage holds the title and body for a notification sent to both iOS and Android.
type NotificationMessage struct {
	Title string
	Body  string
}

// AppleNotification is the full APNs payload.
type AppleNotification struct {
	Aps struct {
		Alert NotificationMessage `json:"alert"`
	} `json:"aps"`
}

// AndroidNotification is the FCM payload.
type AndroidNotification struct {
	Notification NotificationMessage `json:"notification"`
}

// Installation represents a device installation for Azure Notification Hubs.
type Installation struct {
	// InstallationID is a unique identifier for the installation (usually a device ID or UUID).
	// This is used to update or delete installations.
	InstallationID string `json:"installationId"`

	// Platform is the platform type for the device.
	// Valid values: "apns" for iOS, "fcm" for Android.
	Platform string `json:"platform"`

	// PushChannel is the device-specific token to receive notifications.
	// For APNs: the device token from Apple.
	// For FCM: the registration token from Firebase.
	// Ref: https://learn.microsoft.com/en-us/rest/api/notificationhubs/installation#pushchannel
	PushChannel string `json:"pushChannel"`

	// Tags is an optional list of tags to categorize this device.
	// These are used for targeting groups of installations (e.g., "user:123").
	Tags []string `json:"tags,omitempty"`

	// Templates defines push notification templates for the device.
	// This is optional and only needed for advanced templated notifications.
	Templates map[string]Template `json:"templates,omitempty"`
}

// Template is used for advanced push templates (optional).
type Template struct {
	Body string   `json:"body"`
	Tags []string `json:"tags,omitempty"`
}

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

// RegisterDevice registers a device installation with Azure Notification Hubs.
// Read more at: https://learn.microsoft.com/en-us/answers/questions/1324518/sending-notification-registering-device-in-notific.
//
// You use the tags you assign during registration to send notifications, as this is how you target specific devices.
// For example, if you register a device with the tag "user:123", you can send a notification to that device
// by targeting the "user:123" tag.
func RegisterDevice(hubName, namespace, sasToken string, installation Installation) error {
	jsonData, err := json.Marshal(installation)
	if err != nil {
		return fmt.Errorf("failed to marshal installation: %w", err)
	}
	url := fmt.Sprintf("https://%s.servicebus.windows.net/%s/installations/%s?api-version=2020-06", namespace, hubName, installation.InstallationID)
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", sasToken)
	client := HTTPClient
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send registration: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("registration failed: %s", resp.Status)
	}
	return nil
}

// SendNotification sends a cross-platform push notification to all devices for a given user.
// If a user has no devices on a given platform, the error is logged and skipped.
func SendNotification(hubName, namespace, sasToken, userID string, msg NotificationMessage) error {
	if err := sendPlatformNotification(hubName, namespace, sasToken, userID, msg, "apple"); err != nil {
		fmt.Printf("[WARN] APNs notification skipped or failed: %v\n", err)
	}
	if err := sendPlatformNotification(hubName, namespace, sasToken, userID, msg, "gcm"); err != nil {
		fmt.Printf("[WARN] FCM notification skipped or failed: %v\n", err)
	}
	return nil
}

// sendPlatformNotification sends a platform-specific push notification.
func sendPlatformNotification(hubName, namespace, sasToken, userID string, msg NotificationMessage, platform string) error {
	var payload []byte
	var err error

	switch platform {
	case "apple":
		apns := AppleNotification{}
		apns.Aps.Alert = msg
		payload, err = json.Marshal(apns)
	case "gcm":
		fcm := AndroidNotification{Notification: msg}
		payload, err = json.Marshal(fcm)
	default:
		return fmt.Errorf("unsupported platform: %s", platform)
	}

	if err != nil {
		return fmt.Errorf("failed to marshal payload for %s: %w", platform, err)
	}

	url := fmt.Sprintf("https://%s.servicebus.windows.net/%s/messages/?api-version=2020-06", namespace, hubName)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create %s request: %w", platform, err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", sasToken)
	req.Header.Set("ServiceBusNotification-Format", platform)
	req.Header.Set("ServiceBusNotification-Tags", fmt.Sprintf("user:%s", userID))

	client := HTTPClient
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send %s request: %w", platform, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
		return fmt.Errorf("%s notification skipped, no devices found (status %d)", platform, resp.StatusCode)
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("%s notification failed with status %d", platform, resp.StatusCode)
	}
	return nil
}
