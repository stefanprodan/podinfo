package main

import "testing"

func TestInitZap(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "error", "fatal", "panic"} {
		if _, err := initZap(level, nil); err != nil {
			t.Errorf("initZap(%q) returned %v", level, err)
		}
	}

	if _, err := initZap("verbose", nil); err == nil {
		t.Error("initZap(\"verbose\") returned no error")
	}
}
