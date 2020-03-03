package spread_test

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"time"

	"golang.org/x/crypto/ssh"

	. "gopkg.in/check.v1"

	"github.com/snapcore/spread/spread"
)

type mockSystem string

func (ms mockSystem) String() string { return string(ms) }

type clientSuite struct {
	ctx    context.Context
	system mockSystem
}

var _ = Suite(&clientSuite{})

func (s *clientSuite) SetUpTest(c *C) {
	s.ctx = context.Background()
	s.system = mockSystem("some-system")
}

func (s *clientSuite) TestWaitPortUpHappyNoCmd(c *C) {
	connectedCh := make(chan interface{})
	ln, err := net.Listen("tcp", "localhost:0")
	c.Assert(err, IsNil)
	go func() {
		conn, err := ln.Accept()
		c.Assert(err, IsNil)
		conn.Close()
		close(connectedCh)
	}()

	err = spread.WaitPortUp(s.ctx, s.system, ln.Addr().String(), nil)
	c.Assert(err, IsNil)

	// ensure waitPortUp really connected to our listener
	timeout := time.NewTicker(5 * time.Second)
	select {
	case <-connectedCh:
		break
	case <-timeout.C:
		c.Fatalf("timeout waiting for connection")
	}
}

func (s *clientSuite) TestWaitPortUpHappyCmdHappy(c *C) {
	cmd := exec.Command("sleep", "9999")
	cmd.Start()
	defer cmd.Process.Kill()

	connectedCh := make(chan interface{})
	ln, err := net.Listen("tcp", "localhost:0")
	c.Assert(err, IsNil)
	go func() {
		conn, err := ln.Accept()
		c.Assert(err, IsNil)
		conn.Close()
		close(connectedCh)
	}()

	err = spread.WaitPortUp(s.ctx, s.system, ln.Addr().String(), cmd)
	c.Assert(err, IsNil)

	// ensure waitPortUp really connected to our listener
	timeout := time.NewTicker(5 * time.Second)
	select {
	case <-connectedCh:
		break
	case <-timeout.C:
		c.Fatalf("timeout waiting for connection")
	}
}

func (s *clientSuite) TestWaitPortUpHappyCmdFailing(c *C) {
	cmd := exec.Command("false", "hope")
	cmd.Start()

	err := spread.WaitPortUp(s.ctx, s.system, "localhost:0", cmd)
	c.Assert(err, ErrorMatches, `process exited unexpectedly while waiting for address localhost:0 \(wstatus=256\)`)
}

func (s *clientSuite) TestDialOnReboot(c *C) {
	restore := spread.MockSshDial(func(network, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
		time.Sleep(1 * time.Second)
		return nil, fmt.Errorf("cannot connect")
	})
	defer restore()

	cli := spread.MockClient()
	spread.SetWarnTimeout(cli, 50*time.Millisecond)
	spread.SetKillTimeout(cli, 100*time.Millisecond)

	err := spread.DialOnReboot(cli, time.Time{})
	c.Check(err, ErrorMatches, "kill-timeout reached after mock-job reboot request")
}
