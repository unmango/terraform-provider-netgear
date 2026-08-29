// Package fastpath speaks the Broadcom FASTPATH CLI that NETGEAR smart switches
// expose over telnet on port 60000.
//
// The interface is undocumented and disabled by default. It is enabled in the web
// UI under Maintenance > Troubleshooting > Remote Diagnostics, and it carries the
// login password in the clear, so it belongs on a management VLAN only.
package fastpath

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Flow selects the login sequence. Firmware generations differ in how they prompt
// for the username and whether `enable` asks for a password at all.
type Flow string

const (
	// FlowAuto detects the flow from the prompts the switch sends.
	FlowAuto Flow = "auto"

	// FlowModern expects a `User:` prompt and no enable password.
	FlowModern Flow = "modern"

	// FlowLegacy expects the username prompt embedded in the interface
	// configuration banner, and an enable prompt that wants an empty password.
	FlowLegacy Flow = "legacy"
)

// DefaultTimeout applies when Config.Timeout is zero.
const DefaultTimeout = 30 * time.Second

// Config describes how to reach and log in to a switch.
type Config struct {
	Host           string
	Port           int64
	Username       string
	Password       string
	EnablePassword string
	Flow           Flow
	Timeout        time.Duration
}

// Dialer opens a transport to the switch. It exists so tests can supply an
// in-memory connection in place of a TCP one.
type Dialer func(ctx context.Context) (net.Conn, error)

// Client runs commands against a switch. Each call opens its own CLI session,
// because FASTPATH tolerates only a small number of concurrent sessions and a
// long lived telnet connection is easily dropped by the switch. Calls are
// serialized so a single Client never holds two sessions at once.
type Client struct {
	cfg  Config
	dial Dialer
	mu   sync.Mutex
}

// New returns a Client that dials the configured host over TCP.
func New(cfg Config) (*Client, error) {
	if cfg.Host == "" {
		return nil, errors.New("host is required")
	}
	if cfg.Port == 0 {
		cfg.Port = 60000
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = DefaultTimeout
	}
	if cfg.Flow == "" {
		cfg.Flow = FlowAuto
	}

	address := net.JoinHostPort(cfg.Host, strconv.FormatInt(cfg.Port, 10))

	return NewWithDialer(cfg, func(ctx context.Context) (net.Conn, error) {
		var d net.Dialer
		conn, err := d.DialContext(ctx, "tcp", address)
		if err != nil {
			return nil, &ConnectError{Address: address, Err: err}
		}
		return conn, nil
	})
}

// NewWithDialer returns a Client that opens its transport with dial.
func NewWithDialer(cfg Config, dial Dialer) (*Client, error) {
	if dial == nil {
		return nil, errors.New("dialer is required")
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = DefaultTimeout
	}
	if cfg.Flow == "" {
		cfg.Flow = FlowAuto
	}

	return &Client{cfg: cfg, dial: dial}, nil
}

// ConnectError reports a transport failure. A refused connection while the switch
// still answers ICMP is the usual symptom of the CLI being disabled.
type ConnectError struct {
	Address string
	Err     error
}

func (e *ConnectError) Error() string {
	return fmt.Sprintf("connecting to %s: %v", e.Address, e.Err)
}

func (e *ConnectError) Unwrap() error { return e.Err }

// Run executes commands in order and returns the combined session output. It
// stops at the first command the switch rejects, returning the output collected
// so far alongside the error.
func (c *Client) Run(ctx context.Context, cmds ...string) (string, error) {
	if len(cmds) == 0 {
		return "", nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	session, err := c.open(ctx)
	if err != nil {
		return "", err
	}
	defer session.close()

	var out strings.Builder
	for _, cmd := range cmds {
		result, err := session.exec(ctx, cmd)
		out.WriteString(result)
		if err != nil {
			return out.String(), err
		}
	}

	return out.String(), nil
}

// Probe opens a session and logs in without running anything. It verifies the
// switch is reachable, the CLI is enabled, and the credentials and login flow
// work.
func (c *Client) Probe(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	session, err := c.open(ctx)
	if err != nil {
		return err
	}

	session.close()

	return nil
}

// RunningConfig returns the output of `show running-config`.
func (c *Client) RunningConfig(ctx context.Context) (string, error) {
	return c.Run(ctx, "show running-config")
}

// Save writes the running configuration to NVRAM. FASTPATH applies changes live,
// so without this they are lost on reboot.
func (c *Client) Save(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	session, err := c.open(ctx)
	if err != nil {
		return err
	}
	defer session.close()

	return session.save(ctx)
}

func (c *Client) open(ctx context.Context) (*session, error) {
	conn, err := c.dial(ctx)
	if err != nil {
		return nil, err
	}

	s := newSession(conn, c.cfg)
	if err := s.login(ctx); err != nil {
		s.close()
		return nil, err
	}

	return s, nil
}
