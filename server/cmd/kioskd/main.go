// Command kioskd is the kiosk agent: it starts seatd and the Cage compositor
// running Chromium on the mirror's dashboard, and applies the screen state the
// server asks for (rotation, later power) with wlr-randr. It starts programs
// only with fixed argument lists, never through a shell.
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"strconv"
	"syscall"
	"time"
	_ "time/tzdata" // keeps TZ working even if the image lacks zoneinfo

	"github.com/thazzr/mirrornextgen/server/internal/kiosk"
)

const (
	runtimeDir     = "/run/kiosk"
	seatdSocket    = "/run/seatd.sock"
	waylandSocket  = runtimeDir + "/wayland-0"
	seatdTimeout   = 10 * time.Second
	waylandTimeout = 30 * time.Second
	stopGrace      = 10 * time.Second
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Getenv, os.Stdout)
	stop()
	os.Exit(code)
}

func run(ctx context.Context, getenv func(string) string, stdout io.Writer) int {
	log := slog.New(slog.NewJSONHandler(stdout, nil))

	cfg, err := kiosk.Load(getenv)
	if err != nil {
		log.Error("invalid configuration", "error", err)
		return 1
	}
	account, err := lookupAccount(cfg.User)
	if err != nil {
		log.Error("cannot look up kiosk user", "user", cfg.User, "error", err)
		return 1
	}

	httpClient := &http.Client{Timeout: 10 * time.Second}
	log.Info("waiting for the mirror server", "url", cfg.URL)
	if err := kiosk.WaitHealthy(ctx, httpClient, cfg.URL, 2*time.Second, 30*time.Second, log); err != nil {
		log.Info("stopped while waiting for the mirror server", "error", err)
		return 0
	}

	if err := prepareRuntimeDir(account); err != nil {
		log.Error("cannot prepare runtime dir", "error", err)
		return 1
	}

	seatd, err := startProc(log, "seatd", nil, "seatd", "-u", cfg.User, "-g", cfg.User)
	if err != nil {
		log.Error("cannot start seatd", "error", err)
		return 1
	}
	if err := waitForPath(ctx, seatdSocket, seatdTimeout); err != nil {
		log.Error("seatd socket did not appear", "error", err)
		stopProc(seatd, stopGrace, log)
		return 1
	}

	cageArgs := kiosk.CageArgs(kiosk.ChromiumArgs(cfg, kiosk.BrowserURL(cfg)))
	cage, err := startProc(log, "cage", account, "cage", cageArgs...)
	if err != nil {
		log.Error("cannot start cage", "error", err)
		stopProc(seatd, stopGrace, log)
		return 1
	}
	if err := waitForPath(ctx, waylandSocket, waylandTimeout); err != nil {
		log.Error("wayland socket did not appear", "error", err)
		return shutdown(log, 1, cage, seatd)
	}
	log.Info("kiosk is up", "url", kiosk.BrowserURL(cfg), "rotation", cfg.Rotation)

	agentCtx, cancelAgent := context.WithCancel(ctx)
	defer cancelAgent()
	if cfg.Rotation == kiosk.RotationCompositor {
		agent := &kiosk.Agent{
			Fetch: func(c context.Context) (kiosk.State, error) {
				return kiosk.FetchState(c, httpClient, cfg.URL)
			},
			Commander: &userCommander{account: account},
			Log:       log,
		}
		ticker := time.NewTicker(cfg.PollInterval)
		defer ticker.Stop()
		go agent.Run(agentCtx, ticker.C)
	}

	code := supervise(ctx, log, cage, seatd)
	cancelAgent()
	return code
}

// proc is a supervised child process.
type proc interface {
	Name() string
	Done() <-chan struct{} // closed when the process has exited
	Terminate()
	Kill()
}

// supervise waits until a child dies or ctx ends and then stops both children,
// cage first, then seatd. It returns the exit code: 0 for a requested
// shutdown, 1 when a child died on its own (Docker restarts the container).
func supervise(ctx context.Context, log *slog.Logger, cage, seatd proc) int {
	select {
	case <-ctx.Done():
		log.Info("shutting down")
		return shutdown(log, 0, cage, seatd)
	case <-cage.Done():
		log.Error("cage exited")
	case <-seatd.Done():
		log.Error("seatd exited")
	}
	return shutdown(log, 1, cage, seatd)
}

func shutdown(log *slog.Logger, code int, cage, seatd proc) int {
	stopProc(cage, stopGrace, log)
	stopProc(seatd, stopGrace, log)
	return code
}

// stopProc asks the process to end with SIGTERM and kills it after grace.
func stopProc(p proc, grace time.Duration, log *slog.Logger) {
	select {
	case <-p.Done():
		return
	default:
	}
	p.Terminate()
	select {
	case <-p.Done():
	case <-time.After(grace):
		log.Warn("process ignored SIGTERM, killing it", "process", p.Name())
		p.Kill()
		<-p.Done()
	}
}

type execProc struct {
	name string
	cmd  *exec.Cmd
	done chan struct{}
}

func (p *execProc) Name() string          { return p.name }
func (p *execProc) Done() <-chan struct{} { return p.done }
func (p *execProc) Terminate()            { _ = p.cmd.Process.Signal(syscall.SIGTERM) }
func (p *execProc) Kill()                 { _ = p.cmd.Process.Kill() }

// startProc starts a child. asUser nil means it runs as the current user
// (root). Its output is forwarded to the log with the process name.
func startProc(log *slog.Logger, name string, asUser *account, bin string, args ...string) (*execProc, error) {
	// Not CommandContext: shutdown order and grace are managed by supervise.
	cmd := exec.Command(bin, args...)
	cmd.Env = baseEnv()
	if asUser != nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{Credential: asUser.credential()}
	}
	cmd.Stdout = &lineLogger{log: log, name: name}
	cmd.Stderr = &lineLogger{log: log, name: name, stderr: true}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", bin, err)
	}
	p := &execProc{name: name, cmd: cmd, done: make(chan struct{})}
	go func() {
		err := cmd.Wait()
		log.Info("process exited", "process", name, "error", fmt.Sprint(err))
		close(p.done)
	}()
	return p, nil
}

// baseEnv is the environment of cage and its helpers: the runtime dir, the
// seat backend, no input devices (so no cursor) and the optional pass-throughs.
func baseEnv() []string {
	env := []string{
		"PATH=" + os.Getenv("PATH"),
		"XDG_RUNTIME_DIR=" + runtimeDir,
		"LIBSEAT_BACKEND=seatd",
		"WLR_LIBINPUT_NO_DEVICES=1",
		"HOME=/tmp",
	}
	for _, k := range []string{"WLR_RENDERER", "WLR_DRM_DEVICES", "TZ"} {
		if v := os.Getenv(k); v != "" {
			env = append(env, k+"="+v)
		}
	}
	return env
}

type account struct {
	uid, gid uint32
	groups   []uint32
}

func (a *account) credential() *syscall.Credential {
	return &syscall.Credential{Uid: a.uid, Gid: a.gid, Groups: a.groups}
}

func lookupAccount(name string) (*account, error) {
	u, err := user.Lookup(name)
	if err != nil {
		return nil, err
	}
	uid, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("uid %q: %w", u.Uid, err)
	}
	gid, err := strconv.ParseUint(u.Gid, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("gid %q: %w", u.Gid, err)
	}
	a := &account{uid: uint32(uid), gid: uint32(gid)}
	if ids, err := u.GroupIds(); err == nil {
		for _, s := range ids {
			if g, err := strconv.ParseUint(s, 10, 32); err == nil {
				a.groups = append(a.groups, uint32(g))
			}
		}
	}
	return a, nil
}

func prepareRuntimeDir(a *account) error {
	if err := os.MkdirAll(runtimeDir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(runtimeDir, 0o700); err != nil {
		return err
	}
	return os.Chown(runtimeDir, int(a.uid), int(a.gid))
}

// waitForPath polls until path exists, ctx ends or the timeout passes.
func waitForPath(ctx context.Context, path string, timeout time.Duration) error {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return fmt.Errorf("%s did not appear within %s", path, timeout)
		case <-tick.C:
		}
	}
}

// userCommander runs wlr-randr as the kiosk user against the Wayland socket.
type userCommander struct{ account *account }

func (c *userCommander) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(baseEnv(), "WAYLAND_DISPLAY=wayland-0")
	cmd.SysProcAttr = &syscall.SysProcAttr{Credential: c.account.credential()}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, errors.Join(err, errors.New(string(bytes.TrimSpace(stderr.Bytes()))))
	}
	return stdout.Bytes(), nil
}

// lineLogger turns a child's output lines into log records.
type lineLogger struct {
	log    *slog.Logger
	name   string
	stderr bool
	buf    []byte
}

func (l *lineLogger) Write(p []byte) (int, error) {
	l.buf = append(l.buf, p...)
	for {
		i := bytes.IndexByte(l.buf, '\n')
		if i < 0 {
			break
		}
		l.emit(string(l.buf[:i]))
		l.buf = l.buf[i+1:]
	}
	return len(p), nil
}

func (l *lineLogger) emit(line string) {
	if line == "" {
		return
	}
	if l.stderr {
		l.log.Warn(line, "process", l.name, "stream", "stderr")
		return
	}
	l.log.Info(line, "process", l.name, "stream", "stdout")
}
