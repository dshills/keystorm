//go:build darwin

package terminal

import (
	"os"
	"os/exec"
	"syscall"
	"unsafe"
)

// IOCTL constants for macOS.
const (
	TIOCSWINSZ = 0x80087467
	TIOCGPTN   = 0x40045430 // Not used on macOS
	TIOCSPTLCK = 0x40045431 // Not used on macOS
)

// startPTY starts a command with a PTY on macOS.
func startPTY(cmd *exec.Cmd, cols, rows uint16) (PTY, error) {
	// On macOS, we use posix_openpt via /dev/ptmx
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}

	// Get the slave path using TIOCPTYGNAME
	slavePath, err := ptsNameDarwin(master)
	if err != nil {
		master.Close()
		return nil, err
	}

	// Open slave
	slave, err := os.OpenFile(slavePath, os.O_RDWR, 0)
	if err != nil {
		master.Close()
		return nil, err
	}

	// Set initial size
	if err := setWinSizeDarwin(master, cols, rows); err != nil {
		master.Close()
		slave.Close()
		return nil, err
	}

	// Set up the command to use the slave as its controlling terminal
	cmd.Stdin = slave
	cmd.Stdout = slave
	cmd.Stderr = slave

	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setsid = true
	cmd.SysProcAttr.Setctty = true

	// Start the command
	if err := cmd.Start(); err != nil {
		master.Close()
		slave.Close()
		return nil, err
	}

	// Close the slave in the parent process
	slave.Close()

	return &darwinPTY{
		master: master,
	}, nil
}

// darwinPTY implements PTY for macOS.
type darwinPTY struct {
	master *os.File
}

func (p *darwinPTY) File() *os.File {
	return p.master
}

func (p *darwinPTY) Read(buf []byte) (int, error) {
	return p.master.Read(buf)
}

func (p *darwinPTY) Write(data []byte) (int, error) {
	return p.master.Write(data)
}

func (p *darwinPTY) Resize(cols, rows uint16) error {
	return setWinSizeDarwin(p.master, cols, rows)
}

func (p *darwinPTY) Close() error {
	return p.master.Close()
}

// ptsNameDarwin returns the path of the slave PTY on macOS.
func ptsNameDarwin(master *os.File) (string, error) {
	// On macOS, use TIOCPTYGNAME ioctl
	// #define TIOCPTYGNAME _IOC(IOC_OUT, 't', 107, 128)
	const TIOCPTYGNAME = 0x40807467

	var name [128]byte
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		master.Fd(),
		TIOCPTYGNAME,
		uintptr(unsafe.Pointer(&name[0])),
	)
	if errno != 0 {
		return "", errno
	}

	// Find null terminator
	var end int
	for end = 0; end < len(name) && name[end] != 0; end++ {
	}

	return string(name[:end]), nil
}

// setWinSizeDarwin sets the window size on macOS.
func setWinSizeDarwin(f *os.File, cols, rows uint16) error {
	ws := &winSizeDarwin{
		Row: rows,
		Col: cols,
	}
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		f.Fd(),
		TIOCSWINSZ,
		uintptr(unsafe.Pointer(ws)),
	)
	if errno != 0 {
		return errno
	}
	return nil
}

// winSizeDarwin is the structure used by TIOCSWINSZ.
type winSizeDarwin struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}
