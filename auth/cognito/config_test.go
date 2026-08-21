package cognito

import "testing"

func TestSecretHashStable(t *testing.T) {
	got := SecretHash("user@example.com", "clientid", "secret")
	if got == "" {
		t.Fatal("empty hash")
	}
	again := SecretHash("user@example.com", "clientid", "secret")
	if got != again {
		t.Fatal("hash not stable")
	}
	if SecretHash("other", "clientid", "secret") == got {
		t.Fatal("hash should change with username")
	}
}

func TestConfigValidate(t *testing.T) {
	cfg := Config{UserPoolID: "pool", ClientID: "c", Username: "u", Password: "p", Region: "ap-south-1"}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	cfg.Username = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected username required")
	}
}
