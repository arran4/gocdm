package auth

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"
)

type fakePAMHandle struct {
	events     []string
	tty        string
	pamUser    string
	putEnv     []string
	envList    map[string]string
	conv       func(pamConvStyle, string) (string, error)
	authErr    error
	acctErr    error
	setCredErr error
	openErr    error
	closeErr   error
	endErr     error
	onAuth     func(conv func(pamConvStyle, string) (string, error))
	onAcctMgmt func(conv func(pamConvStyle, string) (string, error))
	onSetCred  func(flag pamCredFlag, conv func(pamConvStyle, string) (string, error))
	onOpen     func(conv func(pamConvStyle, string) (string, error))
	onClose    func(conv func(pamConvStyle, string) (string, error))
}

func (f *fakePAMHandle) SetItemTTY(tty string) error {
	f.tty = tty
	f.events = append(f.events, "set_tty:"+tty)
	return nil
}

func (f *fakePAMHandle) GetItemUser() (string, error) {
	f.events = append(f.events, "get_user")
	return f.pamUser, nil
}

func (f *fakePAMHandle) Authenticate() error {
	f.events = append(f.events, "authenticate")
	if f.onAuth != nil {
		f.onAuth(f.conv)
	}
	return f.authErr
}

func (f *fakePAMHandle) AcctMgmt() error {
	f.events = append(f.events, "acct_mgmt")
	if f.onAcctMgmt != nil {
		f.onAcctMgmt(f.conv)
	}
	return f.acctErr
}

func (f *fakePAMHandle) PutEnv(nameval string) error {
	f.putEnv = append(f.putEnv, nameval)
	f.events = append(f.events, "putenv:"+nameval)
	return nil
}

func (f *fakePAMHandle) SetCred(flag pamCredFlag) error {
	name := "establish"
	if flag == credDelete {
		name = "delete"
	}
	f.events = append(f.events, "setcred:"+name)
	if f.onSetCred != nil {
		f.onSetCred(flag, f.conv)
	}
	if flag == credEstablish {
		return f.setCredErr
	}
	return nil
}

func (f *fakePAMHandle) OpenSession() error {
	f.events = append(f.events, "open_session")
	if f.onOpen != nil {
		f.onOpen(f.conv)
	}
	return f.openErr
}

func (f *fakePAMHandle) GetEnvList() (map[string]string, error) {
	f.events = append(f.events, "getenvlist")
	return f.envList, nil
}

func (f *fakePAMHandle) CloseSession() error {
	f.events = append(f.events, "close_session")
	if f.onClose != nil {
		f.onClose(f.conv)
	}
	return f.closeErr
}

func (f *fakePAMHandle) End() error {
	f.events = append(f.events, "end")
	return f.endErr
}

func TestPAMTransactionFullLifecycleAndNotices(t *testing.T) {
	h := &fakePAMHandle{
		pamUser: "alice_canon",
		envList: map[string]string{
			"XDG_RUNTIME_DIR": "/run/user/1001",
			"XDG_SESSION_ID":  "5",
		},
		onAuth: func(conv func(pamConvStyle, string) (string, error)) {
			pw, err := conv(convPromptEchoOff, "Password: ")
			if err != nil || pw != "secret" {
				t.Fatalf("expected password prompt reply, got (%q, %v)", pw, err)
			}
			_, _ = conv(convTextInfo, "  Welcome banner line 1\n  Welcome line 2")
		},
		onAcctMgmt: func(conv func(pamConvStyle, string) (string, error)) {
			_, _ = conv(convErrorMsg, "Password expires in 3 days")
		},
		onOpen: func(conv func(pamConvStyle, string) (string, error)) {
			_, _ = conv(convTextInfo, "  __  __  ___ _____ ____ \n |  \\/  |/ _ \\_   _|  _ \\")
		},
	}

	sess, err := runPAMAuthenticate("alice", "secret", "tty2", func(conv func(pamConvStyle, string) (string, error)) (pamHandle, error) {
		h.conv = conv
		return h, nil
	})
	if err != nil {
		t.Fatalf("runPAMAuthenticate failed: %v", err)
	}

	if sess.Username() != "alice_canon" {
		t.Fatalf("Username() = %q, want alice_canon", sess.Username())
	}

	if err := sess.SetEnv("XDG_SESSION_TYPE", "tty"); err != nil {
		t.Fatalf("SetEnv failed: %v", err)
	}
	if err := sess.OpenSession(); err != nil {
		t.Fatalf("OpenSession failed: %v", err)
	}
	// SetEnv after OpenSession must be rejected.
	if err := sess.SetEnv("LATE_VAR", "1"); err == nil {
		t.Fatal("expected SetEnv after OpenSession to fail")
	}

	notices := sess.DrainNotices()
	var stdout, stderr bytes.Buffer
	FlushNotices(notices, &stdout, &stderr)

	if !strings.Contains(stdout.String(), "  Welcome banner line 1\n  Welcome line 2\n") {
		t.Fatalf("expected indented auth notice preserved on stdout, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "  __  __  ___") {
		t.Fatalf("expected MOTD from OpenSession preserved on stdout, got %q", stdout.String())
	}
	if stderr.String() != "Password expires in 3 days\n" {
		t.Fatalf("expected error notice on stderr, got %q", stderr.String())
	}

	// Second DrainNotices must be empty (no duplicate MOTD).
	if second := sess.DrainNotices(); len(second) != 0 {
		t.Fatalf("expected empty second DrainNotices, got %+v", second)
	}

	env := sess.Env()
	wantEnv := []string{"XDG_RUNTIME_DIR=/run/user/1001", "XDG_SESSION_ID=5"}
	if !reflect.DeepEqual(env, wantEnv) {
		t.Fatalf("Env() = %v, want %v", env, wantEnv)
	}

	if err := sess.CloseSession(); err != nil {
		t.Fatalf("CloseSession failed: %v", err)
	}
	// CloseSession must be idempotent.
	if err := sess.CloseSession(); err != nil {
		t.Fatalf("second CloseSession failed: %v", err)
	}

	wantOrder := []string{
		"set_tty:tty2",
		"authenticate",
		"acct_mgmt",
		"get_user",
		"putenv:XDG_SESSION_TYPE=tty",
		"setcred:establish",
		"open_session",
		"getenvlist",
		"close_session",
		"setcred:delete",
		"end",
	}
	if !reflect.DeepEqual(h.events, wantOrder) {
		t.Fatalf("PAM call order = %v, want %v", h.events, wantOrder)
	}
}

func TestPAMTransactionRollbackOnFailures(t *testing.T) {
	t.Run("auth failure calls End and surfaces notice", func(t *testing.T) {
		h := &fakePAMHandle{
			authErr: errors.New("authentication failure"),
			onAuth: func(conv func(pamConvStyle, string) (string, error)) {
				_, _ = conv(convErrorMsg, "account locked")
			},
		}
		_, err := runPAMAuthenticate("alice", "bad", "tty1", func(conv func(pamConvStyle, string) (string, error)) (pamHandle, error) {
			h.conv = conv
			return h, nil
		})
		if err == nil || !strings.Contains(err.Error(), "account locked") {
			t.Fatalf("expected auth error with PAM notice, got %v", err)
		}
		if !reflect.DeepEqual(h.events, []string{"set_tty:tty1", "authenticate", "end"}) {
			t.Fatalf("events = %v", h.events)
		}
	})

	t.Run("acct mgmt failure calls End and preserves notice", func(t *testing.T) {
		h := &fakePAMHandle{
			acctErr: errors.New("account expired"),
			onAcctMgmt: func(conv func(pamConvStyle, string) (string, error)) {
				_, _ = conv(convErrorMsg, "contact administrator")
			},
		}
		_, err := runPAMAuthenticate("alice", "pw", "tty1", func(conv func(pamConvStyle, string) (string, error)) (pamHandle, error) {
			h.conv = conv
			return h, nil
		})
		if err == nil || !strings.Contains(err.Error(), "contact administrator") {
			t.Fatalf("expected acct error with PAM notice, got %v", err)
		}
		if !reflect.DeepEqual(h.events, []string{"set_tty:tty1", "authenticate", "acct_mgmt", "end"}) {
			t.Fatalf("events = %v", h.events)
		}
	})

	t.Run("OpenSession failure rolls back SetCred and calls End", func(t *testing.T) {
		h := &fakePAMHandle{
			openErr: errors.New("pam_systemd failed"),
		}
		sess, err := runPAMAuthenticate("alice", "pw", "tty1", func(conv func(pamConvStyle, string) (string, error)) (pamHandle, error) {
			h.conv = conv
			return h, nil
		})
		if err != nil {
			t.Fatalf("unexpected auth error: %v", err)
		}
		if err := sess.OpenSession(); err == nil || !strings.Contains(err.Error(), "pam_systemd failed") {
			t.Fatalf("expected OpenSession failure, got %v", err)
		}
		// Subsequent CloseSession is a safe no-op.
		if err := sess.CloseSession(); err != nil {
			t.Fatalf("unexpected CloseSession error after rollback: %v", err)
		}
		want := []string{"set_tty:tty1", "authenticate", "acct_mgmt", "get_user", "setcred:establish", "open_session", "setcred:delete", "end"}
		if !reflect.DeepEqual(h.events, want) {
			t.Fatalf("events = %v, want %v", h.events, want)
		}
	})

	t.Run("CloseSession before OpenSession only calls End", func(t *testing.T) {
		h := &fakePAMHandle{}
		sess, err := runPAMAuthenticate("alice", "pw", "tty1", func(conv func(pamConvStyle, string) (string, error)) (pamHandle, error) {
			h.conv = conv
			return h, nil
		})
		if err != nil {
			t.Fatalf("unexpected auth error: %v", err)
		}
		if err := sess.CloseSession(); err != nil {
			t.Fatalf("CloseSession failed: %v", err)
		}
		want := []string{"set_tty:tty1", "authenticate", "acct_mgmt", "get_user", "end"}
		if !reflect.DeepEqual(h.events, want) {
			t.Fatalf("events = %v, want %v", h.events, want)
		}
	})
}

func TestPAMSequentialLoginsCleanState(t *testing.T) {
	for i := 1; i <= 2; i++ {
		h := &fakePAMHandle{
			envList: map[string]string{"XDG_SESSION_ID": string(rune('0' + i))},
			onOpen: func(conv func(pamConvStyle, string) (string, error)) {
				_, _ = conv(convTextInfo, "MOTD")
			},
		}
		sess, err := runPAMAuthenticate("alice", "pw", "tty1", func(conv func(pamConvStyle, string) (string, error)) (pamHandle, error) {
			h.conv = conv
			return h, nil
		})
		if err != nil {
			t.Fatalf("login %d failed: %v", i, err)
		}
		if err := sess.OpenSession(); err != nil {
			t.Fatalf("open %d failed: %v", i, err)
		}
		if notices := sess.DrainNotices(); len(notices) != 1 || notices[0].Text != "MOTD" {
			t.Fatalf("login %d notices = %+v", i, notices)
		}
		if err := sess.CloseSession(); err != nil {
			t.Fatalf("close %d failed: %v", i, err)
		}
	}
}
