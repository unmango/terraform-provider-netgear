package provider

import (
	"context"
	"errors"
	"sync"
)

// fakeClient records the commands a resource sends and replays canned output.
type fakeClient struct {
	// config is what RunningConfig returns.
	config string

	// runErr is returned by the next Run call, if set.
	runErr error

	mu       sync.Mutex
	commands []string
	saves    int
}

func (c *fakeClient) Run(ctx context.Context, cmds ...string) (string, error) {
	c.mu.Lock()
	c.commands = append(c.commands, cmds...)
	c.mu.Unlock()

	if c.runErr != nil {
		return "", c.runErr
	}

	return "", nil
}

func (c *fakeClient) RunningConfig(ctx context.Context) (string, error) {
	if c.runErr != nil {
		return "", c.runErr
	}

	return c.config, nil
}

func (c *fakeClient) Save(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.saves++

	return nil
}

func (c *fakeClient) sent() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	return append([]string(nil), c.commands...)
}

func (c *fakeClient) saveCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.saves
}

// errSwitch stands in for a switch that rejects whatever it is sent.
var errSwitch = errors.New(`the switch rejected "vlan 10": % Invalid input detected at '^' marker`)
