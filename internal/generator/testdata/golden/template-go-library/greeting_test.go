package sampleapp

import "testing"

func TestGreeting(t *testing.T) {
	if got := Greeting(); got == "" {
		t.Fatal("Greeting() returned an empty string")
	}
}
