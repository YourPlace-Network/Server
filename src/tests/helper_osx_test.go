package tests

import (
	"YourPlace/src/core/host"
	"testing"
)

func TestHelperConnection(t *testing.T) {
	output, err := host.HelperCall("ping")
	if err != nil {
		t.Error(err)
	}
	if output == "pong" {
		t.Log(output)
		return
	}
	t.Error("Didn't get pong reply from helper")
}
func TestHelperRestart(t *testing.T) {
	t.Skip("Skipping restart test")
	output, err := host.HelperCall("restart")
	if err != nil {
		t.Error(err)
		return
	}
	if output == "success" {
		t.Log(output)
		return
	}
	t.Error("Didn't get restart reply from helper")
}
func TestHelperUninstall(t *testing.T) {
	t.Skip("Skipping uninstall test")
	output, err := host.HelperCall("uninstall")
	if err != nil {
		t.Error(err)
	}
	t.Log(output)
}
