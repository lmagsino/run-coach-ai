package garmin

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

// TestFakeLoginHelper is not a test: it is the sign-in helper the tests below
// run, by re-running this test binary with a mode after "--".
func TestFakeLoginHelper(t *testing.T) {
	mode := ""
	for i, a := range os.Args {
		if a == "--" && i+1 < len(os.Args) {
			mode = os.Args[i+1]
		}
	}
	if mode == "" {
		return
	}
	in := bufio.NewScanner(os.Stdin)
	in.Scan()
	var creds map[string]string
	_ = json.Unmarshal(in.Bytes(), &creds)
	switch {
	case os.Getenv("ANTHROPIC_API_KEY") != "":
		fmt.Println(`{"status":"error","kind":"failed","message":"the server's environment leaked"}`)
	case mode == "ok":
		fmt.Println(`{"status":"ok","bundle":"bundle-for-` + creds["email"] + `"}`)
	case mode == "mfa":
		fmt.Println(`{"status":"needs_mfa"}`)
		in.Scan()
		var code map[string]string
		_ = json.Unmarshal(in.Bytes(), &code)
		if code["code"] == "123456" {
			fmt.Println(`{"status":"ok","bundle":"bundle-after-mfa"}`)
		} else {
			fmt.Println(`{"status":"error","kind":"bad_code","message":"wrong code"}`)
		}
	case mode == "refused":
		fmt.Println(`{"status":"error","kind":"bad_credentials","message":"401"}`)
	case mode == "silent":
	}
	os.Exit(0)
}

func fakeLogins(mode string) CommandLogins {
	return CommandLogins{Command: os.Args[0] + " -test.run=^TestFakeLoginHelper$ -- " + mode}
}

func TestLoginWithoutMFA(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-must-not-reach-the-child")
	bundle, session, err := fakeLogins("ok").Start("ana@example.com", "secret")
	if err != nil || session != nil || bundle != "bundle-for-ana@example.com" {
		t.Fatalf("got %q %v %v", bundle, session, err)
	}
}

func TestLoginWithMFA(t *testing.T) {
	bundle, session, err := fakeLogins("mfa").Start("ana@example.com", "secret")
	if err != nil || session == nil || bundle != "" {
		t.Fatalf("start: %q %v %v", bundle, session, err)
	}
	if bundle, err = session.Code("123456"); err != nil || bundle != "bundle-after-mfa" {
		t.Fatalf("code: %q %v", bundle, err)
	}

	_, session, _ = fakeLogins("mfa").Start("ana@example.com", "secret")
	if _, err = session.Code("000000"); err == nil {
		t.Fatal("a wrong code connected")
	}
	if kind, _ := IsLoginError(err); kind != LoginBadCode {
		t.Errorf("wrong code: kind %q", kind)
	}
}

func TestLoginFailures(t *testing.T) {
	for mode, want := range map[string]string{"refused": LoginBadCredentials, "silent": LoginUnavailable} {
		_, _, err := fakeLogins(mode).Start("ana@example.com", "secret")
		if kind, _ := IsLoginError(err); kind != want {
			t.Errorf("%s: kind %q, want %q (%v)", mode, kind, want, err)
		}
	}
	_, _, err := CommandLogins{Command: "no-such-runcoach-helper"}.Start("a", "b")
	if kind, _ := IsLoginError(err); kind != LoginUnavailable {
		t.Errorf("missing helper: kind %q", kind)
	}
}
