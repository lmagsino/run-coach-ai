package garmin

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Why a sign-in failed, for the account page to choose its sentence.
const (
	LoginBadCredentials = "bad_credentials"
	LoginBadCode        = "bad_code"
	LoginBlocked        = "blocked"     // Garmin refused this server (rate limit, Cloudflare)
	LoginUnavailable    = "unavailable" // the helper could not be run or did not answer
	LoginFailed         = "failed"
)

// LoginError is a sign-in that did not produce a bundle.
type LoginError struct{ Kind, Message string }

func (e *LoginError) Error() string { return "garmin login: " + e.Kind + ": " + e.Message }

// LoginTTL bounds a sign-in waiting for its MFA code; Garmin's codes expire
// sooner than this anyway.
const LoginTTL = 5 * time.Minute

// stepTimeout bounds one exchange with Garmin, which takes seconds when it works.
const stepTimeout = 90 * time.Second

// Logins starts sign-ins. Start returns a bundle, or a session waiting for the
// MFA code, or an error.
type Logins interface {
	Start(email, password string) (string, LoginSession, error)
}

// LoginSession is a sign-in waiting for its MFA code.
type LoginSession interface {
	Code(code string) (string, error)
	Close()
}

// CommandLogins runs the sign-in helper (deploy/garmin-mcp/login.py) as a
// child process and speaks its JSON lines. The password goes to the child on
// stdin only, and the child gets none of the server's environment beyond what
// starting a program needs, so the API key never reaches it.
type CommandLogins struct {
	Command string // program and args, whitespace separated
}

type commandLogin struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	lines  chan string
	cancel context.CancelFunc
	once   sync.Once
}

func (c CommandLogins) Start(email, password string) (string, LoginSession, error) {
	fields := strings.Fields(c.Command)
	if len(fields) == 0 {
		return "", nil, &LoginError{LoginUnavailable, "no sign-in command is configured"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), LoginTTL)
	cmd := exec.CommandContext(ctx, fields[0], fields[1:]...)
	cmd.Env = childEnv()
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return "", nil, &LoginError{LoginUnavailable, err.Error()}
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return "", nil, &LoginError{LoginUnavailable, err.Error()}
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return "", nil, &LoginError{LoginUnavailable, err.Error()}
	}
	l := &commandLogin{cmd: cmd, stdin: stdin, lines: make(chan string), cancel: cancel}
	go func() {
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 64*1024), 1<<20)
		for sc.Scan() {
			l.lines <- sc.Text()
		}
		close(l.lines)
	}()

	creds, _ := json.Marshal(map[string]string{"email": email, "password": password})
	bundle, mfa, err := l.send(creds)
	if err != nil || !mfa {
		l.Close()
		return bundle, nil, err
	}
	return "", l, nil
}

func (l *commandLogin) Code(code string) (string, error) {
	defer l.Close()
	msg, _ := json.Marshal(map[string]string{"code": code})
	bundle, mfa, err := l.send(msg)
	if err == nil && mfa {
		return "", &LoginError{LoginFailed, "Garmin asked for a second code"}
	}
	return bundle, err
}

func (l *commandLogin) Close() {
	l.once.Do(func() {
		l.cancel()
		_ = l.stdin.Close()
		go func() {
			for range l.lines { // drain so the reader goroutine ends
			}
			_ = l.cmd.Wait()
		}()
	})
}

// send writes one line and reads the reply: a bundle, a request for the MFA
// code, or an error.
func (l *commandLogin) send(line []byte) (bundle string, mfa bool, err error) {
	if _, err := l.stdin.Write(append(line, '\n')); err != nil {
		return "", false, &LoginError{LoginUnavailable, "the sign-in helper stopped: " + err.Error()}
	}
	var text string
	select {
	case t, ok := <-l.lines:
		if !ok {
			return "", false, &LoginError{LoginUnavailable, "the sign-in helper exited without answering"}
		}
		text = t
	case <-time.After(stepTimeout):
		return "", false, &LoginError{LoginUnavailable, "Garmin did not answer in time"}
	}
	var reply struct {
		Status  string `json:"status"`
		Bundle  string `json:"bundle"`
		Kind    string `json:"kind"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(text), &reply); err != nil {
		return "", false, &LoginError{LoginUnavailable, "the sign-in helper said something unexpected"}
	}
	switch reply.Status {
	case "ok":
		return reply.Bundle, false, nil
	case "needs_mfa":
		return "", true, nil
	case "error":
		return "", false, &LoginError{reply.Kind, reply.Message}
	}
	return "", false, &LoginError{LoginUnavailable, fmt.Sprintf("unknown status %q", reply.Status)}
}

// childEnv passes what starting the helper needs, directly or through docker
// locally, and nothing else from the server's environment.
func childEnv() []string {
	var env []string
	for _, k := range []string{"PATH", "HOME", "LANG", "TMPDIR", "DOCKER_HOST", "DOCKER_CONFIG", "DOCKER_CONTEXT"} {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	return env
}

// IsLoginError reports the kind of a sign-in failure, or LoginFailed.
func IsLoginError(err error) (kind, message string) {
	var le *LoginError
	if errors.As(err, &le) {
		return le.Kind, le.Message
	}
	return LoginFailed, err.Error()
}
