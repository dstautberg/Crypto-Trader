package main

import (
	"testing"
)

func TestDetectKrakenPair_AltnameMatch(t *testing.T) {
	json := `{"result": {"XXBTZUSD": {"altname": "XBTUSD"}, "XETHZUSD": {"altname": "ETHUSD"}}}`
	got := detectKrakenPair("XBT", []byte(json))
	want := "XXBTZUSD"
	if got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestDetectKrakenPair_FallbackKeyContains(t *testing.T) {
	json := `{"result": {"XXBTZUSD": {"altname": "SOMETHING"}, "XETHZUSD": {"altname": "ETHUSD"}}}`
	got := detectKrakenPair("XBT", []byte(json))
	want := "XXBTZUSD"
	if got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestDetectKrakenPair_NoMatch(t *testing.T) {
	json := `{"result": {"XETHZUSD": {"altname": "ETHUSD"}}}`
	got := detectKrakenPair("XBT", []byte(json))
	if got != "" {
		t.Fatalf("expected empty string when no match, got %s", got)
	}
}
