package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode"
)

const (
	deliveryTimeout           = 10 * time.Second
	maxNotificationBodyRunes  = 256
	maxNotificationTitleRunes = 128
)

// Available rejects unsupported hosts before a live session consumes identities.
func Available() error {
	if runtime.GOOS != "darwin" {
		return errors.New("desktop notifications require macOS")
	}
	if os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_CLIENT") != "" || os.Getenv("SSH_TTY") != "" {
		return errors.New("desktop notifications require a local macOS session")
	}
	return nil
}

// Send does not request permission. Explicit doctor tests own the first-run
// permission workflow. Message data travels through JSON stdin, never a shell.
func Send(ctx context.Context, message Message) error {
	if err := Available(); err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	helper, err := helperPath(executable)
	if err != nil {
		return fmt.Errorf("notification helper unavailable: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, deliveryTimeout)
	defer cancel()
	bundle := filepath.Dir(filepath.Dir(filepath.Dir(helper)))
	if err := exec.CommandContext(ctx, registrationCommand, "-f", bundle).Run(); err != nil {
		return fmt.Errorf("register notification helper: %w", err)
	}
	message.Title = cleanTitle(message.Title)
	message.Body = cleanBody(message.Body)
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}
	command := exec.CommandContext(ctx, helper, "--deliver")
	command.Stdin = bytes.NewReader(data)
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("notification submission failed: %w", err)
	}
	return nil
}

func cleanTitle(value string) string {
	title := strings.TrimSpace(cleanText(value, maxNotificationTitleRunes))
	if title == "" {
		return "BambuStand"
	}
	return title
}

func cleanBody(value string) string { return cleanText(value, maxNotificationBodyRunes) }

func cleanText(value string, limit int) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, value)
	runes := []rune(value)
	if len(runes) > limit {
		runes = runes[:limit]
	}
	return string(runes)
}
