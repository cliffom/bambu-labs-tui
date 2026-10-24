package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cliffom/bambu-tui/internal/alertstore"
	"github.com/cliffom/bambu-tui/internal/buildinfo"
	"github.com/cliffom/bambu-tui/internal/cloud"
	"github.com/cliffom/bambu-tui/internal/config"
	"github.com/cliffom/bambu-tui/internal/credentials"
	"github.com/cliffom/bambu-tui/internal/farm"
	"github.com/cliffom/bambu-tui/internal/monitor"
	"github.com/cliffom/bambu-tui/internal/noticestore"
	"github.com/cliffom/bambu-tui/internal/ui"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "bambu-tui:", err)
		os.Exit(1)
	}
}
func run(args []string) error {
	command := "monitor"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		command = args[0]
		args = args[1:]
	}
	flags := flag.NewFlagSet("bambu-tui", flag.ContinueOnError)
	path, err := config.Path()
	if err != nil {
		return err
	}
	configPath := flags.String("config", path, "path to non-secret TOML configuration")
	printPath := flags.Bool("config-path", false, "print the default configuration path")
	demo := flags.Bool("demo", false, "run a simulated H2D/X2D farm without signing in")
	token := flags.Bool("token", false, "login by pasting an existing access token (masked)")
	code := flags.Bool("email-code", false, "login using an emailed verification code")
	sessionOnly := flags.Bool("session", false, "login for this session and open the dashboard without saving")
	version := flags.Bool("version", false, "show version")
	flags.Usage = func() {
		_, _ = fmt.Fprint(flags.Output(), "Usage: bambu-tui [login|logout|doctor] [options]\n\nExamples:\n  bambu-tui --demo\n  bambu-tui login\n  bambu-tui login --email-code\n  bambu-tui login --token --session\n  bambu-tui doctor\n  bambu-tui\n\n")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected arguments; use --help")
	}
	if *printPath {
		fmt.Println(path)
		return nil
	}
	if *version {
		fmt.Println(buildinfo.String())
		return nil
	}
	if command != "monitor" && command != "login" && command != "logout" && command != "doctor" {
		return errors.New("unknown command; use login, logout, doctor, or --help")
	}
	if command != "login" && (*token || *code || *sessionOnly) {
		return errors.New("--token, --email-code, and --session are login options")
	}
	if *token && *code {
		return errors.New("choose --token or --email-code")
	}
	if *demo && command != "monitor" {
		return errors.New("--demo is a dashboard option")
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	store := credentials.Store{Region: cfg.Region}
	api := cloud.New(cfg.Region)
	if command == "doctor" {
		fmt.Println(buildinfo.String())
		fmt.Println("Checking OS credential persistence with a disposable test entry…")
		if err := credentials.Probe(cfg.Region); err != nil {
			return fmt.Errorf("credential persistence check failed: %w; check that your keychain is unlocked (Linux: install secret-tool and start Secret Service). Use login --session to continue without saving", err)
		}
		fmt.Println("Credential save, reload, and deletion succeeded. No Bambu credentials were used.")
		return nil
	}
	if command == "logout" {
		if err := store.Delete(); err != nil {
			return err
		}
		fmt.Println("Saved Bambu session removed for region", cfg.Region)
		return nil
	}
	if *demo {
		return dashboard(ctx, cfg, nil, cloud.Session{}, true)
	}
	if command == "login" {
		session, err := authenticate(ctx, api, *token, *code)
		if err != nil {
			return err
		}
		if !*sessionOnly {
			if err := store.Save(session); err == nil {
				fmt.Println("Bambu session saved in the OS credential store. Run bambu-tui to monitor your printers.")
				return nil
			} else {
				fmt.Fprintf(os.Stderr, "Could not save session: %v. Run bambu-tui doctor for a credential-store check.\n", err)
			}
			fmt.Fprintln(os.Stderr, "OS credential store unavailable. Opening a session-only dashboard; the token will not be saved.")
		}
		return dashboard(ctx, cfg, api, session, false)
	}
	session, err := store.Load()
	if err != nil {
		return fmt.Errorf("%w; run bambu-tui login (or bambu-tui --demo)", err)
	}
	return dashboard(ctx, cfg, api, session, false)
}
func authenticate(ctx context.Context, api *cloud.Client, importToken, emailCode bool) (cloud.Session, error) {
	var token string
	if importToken {
		value, err := prompt(ctx, "Bambu access token", true)
		if err != nil {
			return cloud.Session{}, err
		}
		token = strings.TrimSpace(value)
	} else {
		email, err := prompt(ctx, "Bambu account email", false)
		if err != nil {
			return cloud.Session{}, err
		}
		email = strings.TrimSpace(email)
		if !strings.Contains(email, "@") {
			return cloud.Session{}, errors.New("enter an email account")
		}
		var result cloud.LoginResult
		if emailCode {
			if err := api.SendCode(ctx, email); err != nil {
				return cloud.Session{}, err
			}
			verification, err := prompt(ctx, "Email verification code", true)
			if err != nil {
				return cloud.Session{}, err
			}
			result, err = api.Login(ctx, email, "", strings.TrimSpace(verification))
			if err != nil {
				return cloud.Session{}, err
			}
		} else {
			password, err := prompt(ctx, "Bambu password", true)
			if err != nil {
				return cloud.Session{}, err
			}
			result, err = api.Login(ctx, email, password, "")
			if err != nil {
				return cloud.Session{}, err
			}
		}
		if result.LoginType == "verifyCode" && result.AccessToken == "" {
			// The password login endpoint has already sent the initial email code.
			verification, err := prompt(ctx, "Email verification code (already sent)", true)
			if err != nil {
				return cloud.Session{}, err
			}
			result, err = api.Login(ctx, email, "", strings.TrimSpace(verification))
			if err != nil {
				return cloud.Session{}, err
			}
		}
		if result.LoginType == "tfa" && result.AccessToken == "" {
			verification, err := prompt(ctx, "Authenticator code", true)
			if err != nil {
				return cloud.Session{}, err
			}
			result.AccessToken, err = api.TwoFactor(ctx, result.TFAKey, strings.TrimSpace(verification))
			if err != nil {
				return cloud.Session{}, err
			}
		}
		token = result.AccessToken
	}
	if token == "" {
		return cloud.Session{}, errors.New("login did not return an access token; try login --token")
	}
	// Validate the token and resolve the actual account identity before saving.
	return api.Session(ctx, token)
}
func dashboard(ctx context.Context, cfg config.Config, api *cloud.Client, session cloud.Session, demo bool) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	events := make(chan monitor.Event, 128)
	refresh := make(chan struct{}, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		if demo {
			monitor.Demo(ctx, events, refresh)
		} else {
			monitor.New(api, session).Run(ctx, events, refresh)
		}
	}()
	state := farm.New(cfg.LowFilamentPercent)
	model := ui.New(state, events, refresh, demo).WithAlertPreferences(cfg.Alerts)
	if !demo {
		notices, loadErr := noticestore.New(cfg.Region, session.Username)
		var reviews map[string]farm.Review
		if loadErr == nil {
			reviews, loadErr = notices.Load()
		}
		state.RestoreReviews(reviews)
		if loadErr != nil {
			fmt.Fprintln(os.Stderr, "Notice persistence:", loadErr)
		}
		model = model.WithNoticePersistence(notices.Save, loadErr)
		snoozeStore, snoozeErr := alertstore.New(cfg.Region, session.Username)
		var snoozes map[string]time.Time
		if snoozeErr == nil {
			snoozes, snoozeErr = snoozeStore.Load()
		}
		if snoozeErr != nil {
			fmt.Fprintln(os.Stderr, "Alert persistence:", snoozeErr)
		}
		model = model.WithSnoozePersistence(snoozes, snoozeStore.Save, snoozeErr)
	}
	result, err := tea.NewProgram(model, tea.WithContext(ctx), tea.WithAltScreen()).Run()
	if final, ok := result.(ui.Model); ok && final.HasUnsavedPersistence() {
		fmt.Fprintln(os.Stderr, "Notice or snooze changes were not confirmed saved before exit; changes may be lost.")
	}
	cancel()
	<-done
	if errors.Is(err, tea.ErrProgramKilled) && ctx.Err() != nil {
		return nil
	}
	return err
}

type promptModel struct {
	label           string
	input           textinput.Model
	done, cancelled bool
}

func (m promptModel) Init() tea.Cmd { return textinput.Blink }
func (m promptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			m.done = true
			return m, tea.Quit
		case "enter":
			if m.input.Value() != "" {
				m.done = true
				return m, tea.Quit
			}
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}
func (m promptModel) View() string {
	if m.done {
		if m.cancelled {
			return ""
		}
		return m.label + ": [entered]\n"
	}
	return m.label + ": " + m.input.View() + "\n"
}
func prompt(ctx context.Context, label string, secret bool) (string, error) {
	input := textinput.New()
	input.Focus()
	input.Width = 48
	input.CharLimit = 16384
	if secret {
		input.EchoMode = textinput.EchoPassword
		input.EchoCharacter = '•'
	}
	result, err := tea.NewProgram(promptModel{label: label, input: input}, tea.WithContext(ctx)).Run()
	if err != nil {
		return "", errors.New("cannot read interactive input; use a terminal")
	}
	m := result.(promptModel)
	if m.cancelled {
		return "", errors.New("login cancelled")
	}
	return m.input.Value(), nil
}
