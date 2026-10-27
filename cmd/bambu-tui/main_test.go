package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestInvalidThemeFailsBeforeAuthenticationOrTerminalStartup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("theme_file='missing.json'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"--config", path}, {"--demo", "--config", path}, {"login", "--session", "--config", path}} {
		err := run(args)
		if err == nil || !strings.Contains(err.Error(), "read theme") || !strings.Contains(err.Error(), "missing.json") {
			t.Fatal(args, err)
		}
	}
}

func TestFinishDashboardClassifiesExitBeforeCleanupAndJoinsMonitor(t *testing.T) {
	unexpected := errors.New("unexpected program failure")
	for _, tc := range []struct {
		name           string
		parentCanceled bool
		programErr     error
		want           error
	}{
		{"normal quit", false, nil, nil},
		{"external cancellation", true, fmt.Errorf("%w: %w", tea.ErrProgramKilled, context.Canceled), nil},
		{"unrelated kill", false, tea.ErrProgramKilled, tea.ErrProgramKilled},
		{"unexpected failure", false, fmt.Errorf("%w: %w", tea.ErrProgramKilled, unexpected), unexpected},
		{"cancellation after unrelated failure", true, fmt.Errorf("%w: %w", tea.ErrProgramKilled, unexpected), unexpected},
		{"kill without cancellation cause", true, tea.ErrProgramKilled, tea.ErrProgramKilled},
		{"recovered panic", false, fmt.Errorf("%w: %w", tea.ErrProgramKilled, tea.ErrProgramPanic), tea.ErrProgramPanic},
		{"panic with cancellation", true, errors.Join(tea.ErrProgramKilled, tea.ErrProgramPanic, context.Canceled), tea.ErrProgramPanic},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parent, stop := context.WithCancel(context.Background())
			defer stop()
			ctx, cancel := context.WithCancel(parent)
			defer cancel()
			if tc.parentCanceled {
				stop()
			}
			cleanupStarted, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
			releaseWorker := sync.OnceFunc(func() { close(release) })
			t.Cleanup(releaseWorker)
			go func() { <-ctx.Done(); close(cleanupStarted); <-release; close(done) }()
			result := make(chan error, 1)
			go func() { result <- finishDashboard(ctx, cancel, done, tc.programErr) }()
			select {
			case <-cleanupStarted:
			case <-time.After(time.Second):
				t.Fatal("cleanup did not cancel monitor")
			}
			select {
			case early := <-result:
				t.Fatalf("returned before monitor finished: %v", early)
			default:
			}
			releaseWorker()
			select {
			case err := <-result:
				if tc.want == nil {
					if err != nil {
						t.Fatalf("intentional exit failed: %v", err)
					}
				} else if !errors.Is(err, tc.want) || err != tc.programErr {
					t.Fatalf("program failure was swallowed or replaced: %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("cleanup did not finish")
			}
		})
	}
}

func TestRunTerminalProgramHandlesCancellationBeforeStartup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := runTerminalProgram(ctx, promptModel{}, tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithoutRenderer())
	if !errors.Is(err, tea.ErrProgramKilled) || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation cause lost: %v", err)
	}
}
