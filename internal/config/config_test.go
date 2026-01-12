package config

import "testing"

func TestConfigDefaults(t *testing.T) {
	c := &Config{}
	ApplyDefaults(c)
	if c.Port == 0 {
		t.Error("Expected default port to be set")
	}
}
