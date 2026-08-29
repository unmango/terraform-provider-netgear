package fastpath

import (
	"context"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"
)

// Telnet protocol bytes, from RFC 854.
const (
	iac  = 255
	dont = 254
	do   = 253
	wont = 252
	will = 251
	sb   = 250
	se   = 240
)

var (
	// promptRe matches both the model prompt used by newer firmware, for example
	// "(GS724Tv4) #", and the generic "(Broadcom FASTPATH Switching) #" of older
	// firmware, in either user (>) or privileged (#) mode.
	// Config and interface modes nest another parenthesized name onto the prompt,
	// for example "(GS724Tv4) (Interface 0/7)#", so every group is consumed.
	promptRe = regexp.MustCompile(`\((?:[^)\r\n]*)\)(?:\s*\([^)\r\n]*\))*\s*([#>])\s*$`)

	// enabledPromptRe is promptRe restricted to privileged mode.
	enabledPromptRe = regexp.MustCompile(`\([^)\r\n]*\)(?:\s*\([^)\r\n]*\))*\s*#\s*$`)

	userRe    = regexp.MustCompile(`(?i)(?:user|username|login)\s*:\s*$`)
	passRe    = regexp.MustCompile(`(?i)password\s*:\s*$`)
	moreRe    = regexp.MustCompile(`--More--[^\r\n]*$`)
	confirmRe = regexp.MustCompile(`(?i)\(y/n\)[^\r\n]*$`)

	// cliErrorRe matches the ways FASTPATH reports a rejected command. The forms
	// differ by command: a syntax error marks the offending column with `%`, while
	// the VLAN commands answer with a failure table instead.
	cliErrorRe = regexp.MustCompile(
		`(?im)^\s*(%.*` +
			`|ERROR:.*` +
			`|Command not found.*` +
			`|An invalid .*` +
			`|Incorrect input.*` +
			`|.*failed to be configured.*)$`,
	)
)

// ErrNoMatch reports that the switch sent something none of the expected prompts
// matched before the deadline.
var ErrNoMatch = errors.New("no expected prompt in switch output")

type session struct {
	conn net.Conn
	cfg  Config

	buf   []byte
	raw   []byte
	state int
	verb  byte

	// lastMatch is the prompt text the most recent read stopped at, which is what
	// tells user mode and privileged mode apart.
	lastMatch string

	// timeout bounds a single read. It is raised while saving, which takes far
	// longer than any other command.
	timeout time.Duration
}

const (
	stateNormal = iota
	stateIAC
	stateVerb
	stateSubneg
	stateSubnegIAC
)

func newSession(conn net.Conn, cfg Config) *session {
	return &session{
		conn:    conn,
		cfg:     cfg,
		raw:     make([]byte, 4096),
		timeout: cfg.Timeout,
	}
}

func (s *session) close() {
	_ = s.conn.Close()
}

// login drives whichever of the two login flows the switch presents and leaves
// the session in privileged mode with pagination disabled.
func (s *session) login(ctx context.Context) error {
	_, matched, err := s.readUntil(ctx, userRe, passRe, promptRe)
	if err != nil {
		return fmt.Errorf("waiting for the login prompt: %w", err)
	}

	if matched == 0 {
		if err := s.send(ctx, s.cfg.Username); err != nil {
			return err
		}
		if _, matched, err = s.readUntil(ctx, passRe, promptRe); err != nil {
			return fmt.Errorf("waiting for the password prompt: %w", err)
		}
		// readUntil reports indexes into the set it was given, so shift the
		// password match back onto the same numbering as the first read.
		if matched == 0 {
			matched = 1
		} else {
			matched = 2
		}
	}

	if matched == 1 {
		if err := s.send(ctx, s.cfg.Password); err != nil {
			return err
		}
		if _, _, err = s.readUntil(ctx, promptRe); err != nil {
			return fmt.Errorf("logging in: %w", err)
		}
	}

	if err := s.enable(ctx); err != nil {
		return err
	}

	// Disable pagination so long output such as the running config arrives in one
	// piece rather than behind --More-- prompts.
	if _, err := s.exec(ctx, "terminal length 0"); err != nil {
		return err
	}

	return nil
}

// enable raises the session to privileged mode. Newer firmware does not prompt
// for a password at all, older firmware prompts and accepts an empty one.
func (s *session) enable(ctx context.Context) error {
	if s.enabled() {
		return nil
	}

	if err := s.send(ctx, "enable"); err != nil {
		return err
	}

	_, matched, err := s.readUntil(ctx, passRe, promptRe)
	if err != nil {
		return fmt.Errorf("entering privileged mode: %w", err)
	}

	if matched == 0 {
		if s.cfg.Flow == FlowModern {
			return errors.New("the switch asked for an enable password, which cli_flow = \"modern\" does not expect: set cli_flow to \"auto\" or \"legacy\"")
		}
		if err := s.send(ctx, s.cfg.EnablePassword); err != nil {
			return err
		}
		if _, _, err = s.readUntil(ctx, promptRe); err != nil {
			return fmt.Errorf("entering privileged mode: %w", err)
		}
	}

	if !s.enabled() {
		return errors.New("the switch stayed in user mode after enable")
	}

	return nil
}

// enabled reports whether the prompt the session last stopped at is a privileged one.
func (s *session) enabled() bool {
	return enabledPromptRe.MatchString(s.lastMatch)
}

// exec runs a single command and returns its output, without the echoed command
// line or the trailing prompt.
func (s *session) exec(ctx context.Context, cmd string) (string, error) {
	if err := s.send(ctx, cmd); err != nil {
		return "", err
	}

	var out strings.Builder
	for {
		text, matched, err := s.readUntil(ctx, promptRe, moreRe)
		out.WriteString(text)
		if err != nil {
			return out.String(), fmt.Errorf("running %q: %w", cmd, err)
		}
		if matched == 0 {
			break
		}

		// Page through --More-- by sending a space.
		if err := s.write(ctx, []byte(" ")); err != nil {
			return out.String(), err
		}
	}

	result := trimEcho(out.String(), cmd)
	if failure := cliErrorRe.FindString(result); failure != "" {
		return result, fmt.Errorf("the switch rejected %q: %s", cmd, strings.TrimSpace(failure))
	}

	return result, nil
}

// saveTimeoutFactor stretches the read deadline while NVRAM is written. The
// switch warns the operation may take minutes and stops answering during it.
const saveTimeoutFactor = 10

// save writes the running configuration to NVRAM, answering the confirmation
// prompt the switch asks first.
func (s *session) save(ctx context.Context) error {
	restore := s.timeout
	s.timeout = s.timeout * saveTimeoutFactor
	defer func() { s.timeout = restore }()

	if err := s.send(ctx, "copy system:running-config nvram:startup-config"); err != nil {
		return err
	}

	text, matched, err := s.readUntil(ctx, promptRe, confirmRe)
	if err != nil {
		return fmt.Errorf("saving the configuration: %w", err)
	}

	if matched == 1 {
		if err := s.send(ctx, "y"); err != nil {
			return err
		}
		if text, _, err = s.readUntil(ctx, promptRe); err != nil {
			return fmt.Errorf("saving the configuration: %w", err)
		}
	}

	if failure := cliErrorRe.FindString(text); failure != "" {
		return fmt.Errorf("saving the configuration: %s", strings.TrimSpace(failure))
	}

	return nil
}

// send writes a line to the switch. FASTPATH expects CRLF line endings.
func (s *session) send(ctx context.Context, line string) error {
	return s.write(ctx, []byte(line+"\r\n"))
}

func (s *session) write(ctx context.Context, b []byte) error {
	if err := s.conn.SetWriteDeadline(s.deadline(ctx)); err != nil {
		return err
	}

	_, err := s.conn.Write(b)
	return err
}

// readUntil reads until one of exps matches the tail of the buffer. It returns
// everything read up to and including the match, with the matched text removed,
// and the index of the expression that matched.
func (s *session) readUntil(ctx context.Context, exps ...*regexp.Regexp) (string, int, error) {
	for {
		for i, exp := range exps {
			if loc := exp.FindIndex(s.buf); loc != nil {
				text := string(s.buf[:loc[0]])
				s.lastMatch = string(s.buf[loc[0]:loc[1]])
				s.buf = s.buf[loc[1]:]
				return text, i, nil
			}
		}

		if err := s.fill(ctx); err != nil {
			text := string(s.buf)
			s.buf = nil
			if errors.Is(err, context.DeadlineExceeded) || isTimeout(err) {
				return text, -1, fmt.Errorf("%w after %s", ErrNoMatch, s.timeout)
			}
			return text, -1, err
		}
	}
}

// fill reads one chunk from the connection, answering telnet negotiation and
// appending the remaining application bytes to the buffer.
func (s *session) fill(ctx context.Context) error {
	if err := s.conn.SetReadDeadline(s.deadline(ctx)); err != nil {
		return err
	}

	n, err := s.conn.Read(s.raw)
	if n > 0 {
		if perr := s.process(ctx, s.raw[:n]); perr != nil {
			return perr
		}
	}

	return err
}

// process strips telnet negotiation out of chunk, refusing every option the
// switch offers or requests, and appends what is left to the buffer.
func (s *session) process(ctx context.Context, chunk []byte) error {
	var replies []byte

	for _, b := range chunk {
		switch s.state {
		case stateNormal:
			if b == iac {
				s.state = stateIAC
				continue
			}
			s.buf = append(s.buf, b)
		case stateIAC:
			switch b {
			case iac:
				// An escaped 255 byte.
				s.buf = append(s.buf, b)
				s.state = stateNormal
			case will, wont, do, dont:
				s.verb = b
				s.state = stateVerb
			case sb:
				s.state = stateSubneg
			default:
				s.state = stateNormal
			}
		case stateVerb:
			switch s.verb {
			case will, wont:
				replies = append(replies, iac, dont, b)
			case do, dont:
				replies = append(replies, iac, wont, b)
			}
			s.state = stateNormal
		case stateSubneg:
			if b == iac {
				s.state = stateSubnegIAC
			}
		case stateSubnegIAC:
			if b == se {
				s.state = stateNormal
			} else {
				s.state = stateSubneg
			}
		}
	}

	if len(replies) == 0 {
		return nil
	}

	return s.write(ctx, replies)
}

func (s *session) deadline(ctx context.Context) time.Time {
	deadline := time.Now().Add(s.timeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		return ctxDeadline
	}
	return deadline
}

func isTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

// trimEcho drops the command the switch echoes back before its output.
func trimEcho(output, cmd string) string {
	trimmed := strings.TrimLeft(output, "\r\n")
	if rest, ok := strings.CutPrefix(trimmed, cmd); ok {
		trimmed = rest
	}
	return strings.Trim(trimmed, "\r\n")
}
