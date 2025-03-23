package azurepush_test

import (
	"strings"
	"testing"
	"time"

	"github.com/kataras/azurepush"
)

func TestGenerateSASToken(t *testing.T) {
	uri := "https://mynamespace.servicebus.windows.net/myhub"
	keyName := "DefaultFullSharedAccessSignature"
	key := "YWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWE=" // base64-encoded dummy

	token, err := azurepush.GenerateSASToken(uri, keyName, key, time.Hour)
	if err != nil {
		t.Fatalf("expected no error generating SAS token, got: %v", err)
	}

	if !strings.Contains(token, "SharedAccessSignature") {
		t.Error("expected token to contain 'SharedAccessSignature'")
	}
	if !strings.Contains(token, "sig=") || !strings.Contains(token, "se=") || !strings.Contains(token, "skn=") {
		t.Error("expected token to contain sig, se, and skn parameters")
	}

	// t.Log(token)
	// err = azurepush.ValidateToken(context.Background(), http.DefaultClient, "...namespace", "...hub", token)
	// if err != nil {
	// 	t.Fatalf("expected no error validating token, got: %v", err)
	// }
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
