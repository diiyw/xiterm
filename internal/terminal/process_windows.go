//go:build windows

package terminal

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"unsafe"

	"github.com/charmbracelet/x/conpty"
	"github.com/diiyw/xiterm/internal/config"
	"golang.org/x/sys/windows"
)

type windowsProcess struct {
	*conpty.ConPty
	handle windows.Handle
	job    windows.Handle
}

func startProcess(profile config.Profile, cols, rows int) (process, error) {
	// Let exec.LookPath resolve PowerShell/cmd/custom profiles through PATH.
	path, err := exec.LookPath(profile.Shell)
	if err != nil {
		return nil, err
	}
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	console, err := conpty.New(cols, rows, 0)
	if err != nil {
		windows.CloseHandle(job)
		return nil, fmt.Errorf("ConPTY requires Windows 10 1809 or later: %w", err)
	}
	_, handle, err := console.Spawn(path, append([]string{path}, profile.Args...), &syscall.ProcAttr{Dir: profile.Directory, Env: environment(os.Environ(), profile.Env)})
	if err != nil {
		console.Close()
		windows.CloseHandle(job)
		return nil, err
	}
	processHandle := windows.Handle(handle)
	if err = windows.AssignProcessToJobObject(job, processHandle); err != nil {
		windows.TerminateProcess(processHandle, 1)
		windows.CloseHandle(processHandle)
		console.Close()
		windows.CloseHandle(job)
		return nil, fmt.Errorf("assign terminal job: %w", err)
	}
	return &windowsProcess{ConPty: console, handle: processHandle, job: job}, nil
}
func (p *windowsProcess) Wait() error {
	defer windows.CloseHandle(p.handle)
	if _, err := windows.WaitForSingleObject(p.handle, windows.INFINITE); err != nil {
		return err
	}
	var code uint32
	if err := windows.GetExitCodeProcess(p.handle, &code); err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("exit status %d", code)
	}
	return nil
}
func (p *windowsProcess) Close() error {
	// Closing the job kills its shell and descendants. Session's output reader
	// keeps draining until ClosePseudoConsole completes, avoiding pipe deadlocks.
	_ = windows.CloseHandle(p.job)
	return p.ConPty.Close()
}
