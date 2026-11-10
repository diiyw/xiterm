//go:build linux || darwin

package terminal

import (
	"fmt"
	"os"
	"os/exec"
	"sync/atomic"
	"syscall"

	"github.com/creack/pty"
	"github.com/diiyw/xiterm/internal/config"
	"golang.org/x/sys/unix"
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
	// pty's setup ioctls call File.Fd, which puts the descriptor in blocking
	// mode. Reopen a duplicate as a pollable Go file so Close can interrupt a
	// blocked read/write on both Linux and macOS.
	fd, err := unix.FcntlInt(file.Fd(), unix.F_DUPFD_CLOEXEC, 0)
	if err == nil {
		err = unix.SetNonblock(fd, true)
	}
	if err != nil {
		if fd >= 0 {
			_ = unix.Close(fd)
		}
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		_ = file.Close()
		_ = cmd.Wait()
		return nil, fmt.Errorf("make PTY pollable: %w", err)
	}
	pollable := os.NewFile(uintptr(fd), file.Name())
	_ = file.Close()
	return &unixProcess{File: pollable, cmd: cmd}, nil
}
func (p *unixProcess) Resize(cols, rows int) error {
	// SyscallConn preserves nonblocking mode; File.Fd would disable it again.
	raw, err := p.File.SyscallConn()
	if err != nil {
		return err
	}
	var resizeErr error
	if err := raw.Control(func(fd uintptr) {
		resizeErr = unix.IoctlSetWinsize(int(fd), unix.TIOCSWINSZ, &unix.Winsize{Col: uint16(cols), Row: uint16(rows)})
	}); err != nil {
		return err
	}
	return resizeErr
}
func (p *unixProcess) Wait() error { err := p.cmd.Wait(); p.exited.Store(true); return err }
func (p *unixProcess) Close() error {
	if !p.exited.Load() {
		_ = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGHUP)
		_ = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL)
	}
	return p.File.Close()
}
