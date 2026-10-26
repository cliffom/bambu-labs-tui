package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
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
	"github.com/cliffom/bambu-tui/internal/notifications"
	"github.com/cliffom/bambu-tui/internal/notificationstate"
	"github.com/cliffom/bambu-tui/internal/ui"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "bambu-tui:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	command := commandMonitor
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
	notify := flags.Bool("notify", false, "doctor: submit a synthetic desktop notification (macOS)")
	sound := flags.Bool("sound", false, "doctor --notify: request a notification sound")
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
	if command != commandMonitor && command != commandLogin && command != commandLogout && command != commandDoctor {
		return errors.New("unknown command; use login, logout, doctor, or --help")
	}
	if command != commandLogin && (*token || *code || *sessionOnly) {
		return errors.New("--token, --email-code, and --session are login options")
	}
	if *token && *code {
		return errors.New("choose --token or --email-code")
	}
	if *demo && command != commandMonitor {
		return errors.New("--demo is a dashboard option")
	}
	if command != commandDoctor && (*notify || *sound) {
		return errors.New("--notify and --sound are doctor options")
	}
	if *sound && !*notify {
		return errors.New("--sound requires doctor --notify")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if command == commandDoctor && *notify {
		return notificationDoctor(ctx, os.Stdout, *sound, notifications.Test)
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	store := credentials.Store{Region: cfg.Region}
	api := cloud.New(cfg.Region)
	if command == commandDoctor {
		fmt.Println(buildinfo.String())
		fmt.Println("Checking OS credential persistence with a disposable test entry…")
		if err := credentials.Probe(cfg.Region); err != nil {
			return fmt.Errorf("credential persistence check failed: %w; check that your keychain is unlocked (Linux: install secret-tool and start Secret Service). Use login --session to continue without saving", err)
		}
		fmt.Println("Credential save, reload, and deletion succeeded. No Bambu credentials were used.")
		return nil
	}
	if command == commandLogout {
		if err := store.Delete(); err != nil {
			return err
		}
		fmt.Println("Saved Bambu session removed for region", cfg.Region)
		return nil
	}
	if *demo {
		return dashboard(ctx, *configPath, cfg, nil, cloud.Session{}, true)
	}
	if command == commandLogin {
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
		return dashboard(ctx, *configPath, cfg, api, session, false)
	}
	session, err := store.Load()
	if err != nil {
		return fmt.Errorf("%w; run bambu-tui login (or bambu-tui --demo)", err)
	}
	return dashboard(ctx, *configPath, cfg, api, session, false)
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
		if result.LoginType == cloud.LoginVerifyCode && result.AccessToken == "" {
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
		if result.LoginType == cloud.LoginTFA && result.AccessToken == "" {
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

func dashboard(ctx context.Context, configPath string, cfg config.Config, api *cloud.Client, session cloud.Session, demo bool) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	events := make(chan monitor.Event, eventBufferSize)
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
	model := ui.New(state, events, refresh, demo).WithAlertPreferences(cfg.Alerts).WithConfigPersistence(configPath, func(rules config.AlertRules) error {
		return config.SaveNotificationDefaults(configPath, rules)
	}).WithNotificationTest(ctx, notifications.Test)
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
	{ // Prepare the client; the UI starts ledger observation only after opt-in.
		var worker *notifications.Worker
		var ledger notificationstate.Store
		setupErr := notifications.Available()
		cleanup := func() {}
		if setupErr == nil {
			if demo {
				var dir string
				dir, setupErr = os.MkdirTemp("", "bambu-tui-demo-notifications-*")
				if setupErr == nil {
					ledger = notificationstate.Store{Path: filepath.Join(dir, "state.json")}
					cleanup = func() { _ = os.RemoveAll(dir) }
				}
			} else {
				ledger, setupErr = notificationstate.New(cfg.Region, session.Username)
			}
		}
		if setupErr == nil {
			worker = notifications.NewWorker(ctx, ledger, notifications.Send)
			model = model.WithNotifications(worker, nil)
		} else {
			model = model.WithNotifications(nil, setupErr)
		}
		defer func() {
			if worker != nil {
				worker.Stop()
				select {
				case <-worker.Done():
				case <-time.After(notificationShutdownTimeout):
					fmt.Fprintln(os.Stderr, "Desktop notification shutdown unconfirmed; no new submissions will start.")
					return
				}
				if problem := worker.Status().Error; problem != "" {
					fmt.Fprintln(os.Stderr, problem)
				}
			}
			cleanup()
		}()
	}
	// The CLI context owns SIGINT/SIGTERM. A second handler can block sending
	// a quit message after cancellation has stopped Bubble Tea's event loop.
	result, err := runTerminalProgram(ctx, model, tea.WithAltScreen())
	if final, ok := result.(ui.Model); ok && final.HasUnsavedPersistence() {
		fmt.Fprintln(os.Stderr, "Config, notice, or snooze changes were not confirmed saved before exit; changes may be lost.")
	}
	return finishDashboard(ctx, cancel, done, err)
}

// runTerminalProgram gives the CLI context sole ownership of termination
// signals. Request a graceful quit on cancellation: Bubble Tea v1's hard-kill
// path skips joining its input reader before closing cancelreader's descriptors.
func runTerminalProgram(ctx context.Context, model tea.Model, options ...tea.ProgramOption) (tea.Model, error) {
	program := tea.NewProgram(model, append(options, tea.WithoutSignalHandler())...)
	quitSent := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { program.Quit(); close(quitSent) })
	result, err := program.Run()
	if !stop() {
		<-quitSent
	}
	if err == nil && ctx.Err() != nil {
		// Preserve the cancellation result expected by dashboard/prompt callers.
		err = fmt.Errorf("%w: %w", tea.ErrProgramKilled, ctx.Err())
	}
	return result, err
}

// Capture the program's cancellation cause before cleanup cancels the session.
// Only a cancellation reported by Bubble Tea itself is an intentional exit.
func finishDashboard(ctx context.Context, cancel context.CancelFunc, done <-chan struct{}, err error) error {
	cause := ctx.Err()
	cancel()
	<-done
	if errors.Is(err, tea.ErrProgramKilled) && cause != nil && errors.Is(err, cause) && !errors.Is(err, tea.ErrProgramPanic) {
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
		case keyQuit, keyEscape:
			m.cancelled = true
			m.done = true
			return m, tea.Quit
		case keyEnter:
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
	input.Width = promptWidth
	input.CharLimit = promptCharacterLimit
	if secret {
		input.EchoMode = textinput.EchoPassword
		input.EchoCharacter = '•'
	}
	result, err := runTerminalProgram(ctx, promptModel{label: label, input: input})
	if err != nil {
		return "", errors.New("cannot read interactive input; use a terminal")
	}
	m := result.(promptModel)
	if m.cancelled {
		return "", errors.New("login cancelled")
	}
	return m.input.Value(), nil
}
