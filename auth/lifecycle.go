package auth

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

type pamConvStyle int

const (
	convPromptEchoOff pamConvStyle = iota
	convPromptEchoOn
	convErrorMsg
	convTextInfo
)

type pamCredFlag int

const (
	credEstablish pamCredFlag = iota
	credDelete
)

type pamHandle interface {
	SetItemTTY(tty string) error
	GetItemUser() (string, error)
	Authenticate() error
	AcctMgmt() error
	PutEnv(nameval string) error
	SetCred(flag pamCredFlag) error
	OpenSession() error
	GetEnvList() (map[string]string, error)
	CloseSession() error
	End() error
}

type noticeBuffer struct {
	mu      sync.Mutex
	notices []Notice
}

func (b *noticeBuffer) Add(style NoticeStyle, msg string) {
	if strings.TrimSpace(msg) == "" {
		return
	}
	b.mu.Lock()
	b.notices = append(b.notices, Notice{Style: style, Text: msg})
	b.mu.Unlock()
}

func (b *noticeBuffer) Drain() []Notice {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.notices) == 0 {
		return nil
	}
	out := make([]Notice, len(b.notices))
	copy(out, b.notices)
	b.notices = nil
	return out
}

func (b *noticeBuffer) FormatForError() string {
	drained := b.Drain()
	if len(drained) == 0 {
		return ""
	}
	parts := make([]string, 0, len(drained))
	for _, n := range drained {
		if trimmed := strings.TrimSpace(n.Text); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return strings.Join(parts, "; ")
}

type managedLoginSession struct {
	mu              sync.Mutex
	handle          pamHandle
	username        string
	notices         *noticeBuffer
	credEstablished bool
	sessionOpened   bool
	closed          bool
}

func (s *managedLoginSession) Username() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.username
}

func (s *managedLoginSession) SetEnv(key, value string) error {
	if key == "" || strings.ContainsAny(key, "=\x00") {
		return fmt.Errorf("invalid PAM environment key %q", key)
	}
	if strings.ContainsRune(value, '\x00') {
		return fmt.Errorf("invalid PAM environment value for %q", key)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("cannot set PAM environment on closed session")
	}
	if s.sessionOpened {
		return fmt.Errorf("cannot set PAM environment %q after session is opened", key)
	}
	return s.handle.PutEnv(key + "=" + value)
}

func (s *managedLoginSession) OpenSession() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("cannot open closed PAM session")
	}
	if s.sessionOpened {
		return fmt.Errorf("PAM session is already open")
	}

	if err := s.handle.SetCred(credEstablish); err != nil {
		msg := s.notices.FormatForError()
		endErr := s.handle.End()
		s.closed = true
		return formatStageError("PAM establish credentials failed", err, msg, endErr)
	}
	s.credEstablished = true

	if err := s.handle.OpenSession(); err != nil {
		msg := s.notices.FormatForError()
		delErr := s.handle.SetCred(credDelete)
		endErr := s.handle.End()
		s.credEstablished = false
		s.closed = true
		return formatStageError("PAM open session failed", err, msg, errors.Join(delErr, endErr))
	}
	s.sessionOpened = true
	return nil
}

func (s *managedLoginSession) Env() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.handle == nil {
		return nil
	}
	envMap, err := s.handle.GetEnvList()
	if err != nil || len(envMap) == 0 {
		return nil
	}
	keys := make([]string, 0, len(envMap))
	for k := range envMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	env := make([]string, 0, len(keys))
	for _, k := range keys {
		env = append(env, k+"="+envMap[k])
	}
	return env
}

func (s *managedLoginSession) DrainNotices() []Notice {
	if s.notices == nil {
		return nil
	}
	return s.notices.Drain()
}

func (s *managedLoginSession) CloseSession() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.handle == nil {
		return nil
	}

	var errs []error
	if s.sessionOpened {
		if err := s.handle.CloseSession(); err != nil {
			errs = append(errs, fmt.Errorf("PAM close session: %w", err))
		}
	}
	if s.credEstablished {
		if err := s.handle.SetCred(credDelete); err != nil {
			errs = append(errs, fmt.Errorf("PAM delete credentials: %w", err))
		}
	}
	if err := s.handle.End(); err != nil {
		errs = append(errs, fmt.Errorf("PAM end: %w", err))
	}

	s.sessionOpened = false
	s.credEstablished = false
	s.closed = true
	return errors.Join(errs...)
}

func formatStageError(prefix string, stageErr error, noticeText string, cleanupErr error) error {
	err := fmt.Errorf("%s: %w", prefix, stageErr)
	if cleanupErr != nil {
		err = fmt.Errorf("%w (cleanup error: %v)", err, cleanupErr)
	}
	if noticeText != "" {
		err = fmt.Errorf("%w: %s", err, noticeText)
	}
	return err
}

func runPAMAuthenticate(
	username, password, tty string,
	start func(conv func(pamConvStyle, string) (string, error)) (pamHandle, error),
) (LoginSession, error) {
	notices := &noticeBuffer{}
	conv := func(style pamConvStyle, msg string) (string, error) {
		switch style {
		case convPromptEchoOff:
			return password, nil
		case convPromptEchoOn:
			return username, nil
		case convTextInfo:
			notices.Add(NoticeInfo, msg)
			return "", nil
		case convErrorMsg:
			notices.Add(NoticeError, msg)
			return "", nil
		default:
			return "", fmt.Errorf("unsupported PAM style: %v", style)
		}
	}

	h, err := start(conv)
	if err != nil {
		return nil, err
	}

	if tty != "" {
		_ = h.SetItemTTY(tty)
	}

	if err := h.Authenticate(); err != nil {
		endErr := h.End()
		return nil, formatStageError("PAM authentication failed", err, notices.FormatForError(), endErr)
	}

	if err := h.AcctMgmt(); err != nil {
		endErr := h.End()
		return nil, formatStageError("account management failed", err, notices.FormatForError(), endErr)
	}

	canonUser := username
	if pamUser, err := h.GetItemUser(); err == nil && strings.TrimSpace(pamUser) != "" {
		canonUser = strings.TrimSpace(pamUser)
	}

	return &managedLoginSession{
		handle:   h,
		username: canonUser,
		notices:  notices,
	}, nil
}
