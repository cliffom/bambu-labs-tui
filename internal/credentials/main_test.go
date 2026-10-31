package credentials

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	root, err := os.MkdirTemp("", "bambustand-credentials-test-*")
	if err != nil {
		panic(err)
	}
	if err := os.Setenv("HOME", root); err != nil {
		panic(err)
	}
	if err := os.Setenv("XDG_CONFIG_HOME", root); err != nil {
		panic(err)
	}
	code := m.Run()
	_ = os.RemoveAll(root)
	os.Exit(code)
}
