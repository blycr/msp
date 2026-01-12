package server

import "testing"

func TestServerNew(t *testing.T) {
	s := New("config.json")
	if s == nil {
		t.Fatal("Expected server New to not be nil")
	}
}
