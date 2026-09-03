package main

import "testing"

func TestBuildSecrets(t *testing.T) {
	secrets := make(buildSecrets)
	if err := secrets.Set("id=github_token,env=HIVE_GITHUB_TOKEN"); err != nil {
		t.Fatal(err)
	}
	if got, want := secrets["github_token"], "HIVE_GITHUB_TOKEN"; got != want {
		t.Fatalf("secret environment reference = %q, want %q", got, want)
	}
	if got, want := secrets.String(), "id=github_token,env=HIVE_GITHUB_TOKEN"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

func TestBuildSecretsRejectValues(t *testing.T) {
	for _, value := range []string{"", "github_token", "id=", "id=github_token", "env=TOKEN", "id=github_token,env=NOT-AN-ENV", "id=github_token,value=secret"} {
		secrets := make(buildSecrets)
		if err := secrets.Set(value); err == nil {
			t.Errorf("Set(%q) succeeded, want error", value)
		}
	}
}
