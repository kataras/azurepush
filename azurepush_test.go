package azurepush_test

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/kataras/azurepush"
)

// Token.

func TestGenerateSASToken(t *testing.T) {
	uri := "https://mynamespace.servicebus.windows.net/myhub"
	keyName := "DefaultFullSharedAccessSignature"
	key := "YWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWE=" // base64-encoded dummy
	token, err := azurepush.GenerateSASToken(uri, keyName, key, 1*time.Hour)
	if err != nil {
		t.Fatalf("expected no error generating SAS token, got: %v", err)
	}

	if !strings.Contains(token, "SharedAccessSignature") {
		t.Error("expected token to contain 'SharedAccessSignature'")
	}
	if !strings.Contains(token, "sig=") || !strings.Contains(token, "se=") || !strings.Contains(token, "skn=") {
		t.Error("expected token to contain sig, se, and skn parameters")
	}
}

func TestTokenManager_AutoRefresh(t *testing.T) {
	cfg := azurepush.Configuration{
		HubName:       "myhub",
		Namespace:     "mynamespace",
		KeyName:       "DefaultFullSharedAccessSignature",
		KeyValue:      "YWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWE=", // dummy
		TokenValidity: time.Second * 1,
	}
	tm := azurepush.NewTokenManager(cfg)

	token1, err := tm.GetToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	time.Sleep(2 * time.Second)
	token2, err := tm.GetToken()
	if err != nil {
		t.Fatalf("unexpected error after refresh: %v", err)
	}

	if token1 == token2 {
		t.Error("expected different tokens after expiration, got same")
	}
}

// Notifications.

// mockHTTPClient returns a custom HTTP client with the given handler.
func mockHTTPClient(handler func(r *http.Request) *http.Response) *http.Client {
	return &http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return handler(r), nil
		}),
	}
}

type roundTripperFunc func(r *http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestRegisterDevice_Mocked(t *testing.T) {
	// Mock successful 200 OK response
	client := mockHTTPClient(func(r *http.Request) *http.Response {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("{}")),
			Header:     make(http.Header),
		}
	})

	old := azurepush.HTTPClient
	azurepush.HTTPClient = client
	defer func() { azurepush.HTTPClient = old }()

	installation := azurepush.Installation{
		InstallationID: "test-device",
		Platform:       "fcm",
		PushChannel:    "mock-token",
		Tags:           []string{"user:42"},
	}

	err := azurepush.RegisterDevice("hub", "namespace", "token", installation)
	if err != nil {
		t.Fatalf("expected no error from RegisterDevice, got: %v", err)
	}
}

func TestSendNotification_Mocked(t *testing.T) {
	calls := 0
	client := mockHTTPClient(func(r *http.Request) *http.Response {
		calls++
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("{}")),
			Header:     make(http.Header),
		}
	})

	old := azurepush.HTTPClient
	azurepush.HTTPClient = client
	defer func() { azurepush.HTTPClient = old }()

	msg := azurepush.NotificationMessage{Title: "Hi", Body: "Hello"}
	err := azurepush.SendNotification("hub", "namespace", "token", "42", msg)
	if err != nil {
		t.Fatalf("expected no error from SendNotification, got: %v", err)
	}

	if calls != 2 {
		t.Errorf("expected 2 calls (one per platform), got: %d", calls)
	}
}
