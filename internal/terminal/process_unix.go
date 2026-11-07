//go:build linux || darwin

package terminal

import (
	"os"
	"os/exec"
	"sync/atomic"
	"syscall"

	"github.com/creack/pty"
	"github.com/diiyw/xiterm/internal/config"
)

type unixProcess struct {
	*os.File
	cmd    *exec.Cmd
	exited atomic.Bool
}

func startProcess(profile config.Profile, cols, rows int) (process, error) {
	cmd := exec.Command(profile.Shell, profile.Args...)
	cmd.Dir = profile.Directory
	cmd.Env = environment(os.Environ(), profile.Env)
	file, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
	if err != nil {
		return nil, err
	}
	return &unixProcess{File: file, cmd: cmd}, nil
}
func (p *unixProcess) Resize(cols, rows int) error {
	return pty.Setsize(p.File, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
}
func (p *unixProcess) Wait() error { err := p.cmd.Wait(); p.exited.Store(true); return err }
func (p *unixProcess) Close() error {
	if !p.exited.Load() {
		_ = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGHUP)
		_ = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL)
	}
	return p.File.Close()
}
