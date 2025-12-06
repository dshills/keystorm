//go:build linux

package terminal

import (
	"os"
	"os/exec"
	"syscall"
	"unsafe"
)

// startPTY starts a command with a PTY on Unix systems.
func startPTY(cmd *exec.Cmd, cols, rows uint16) (PTY, error) {
	// Open a PTY master
	master, slave, err := openPTY()
	if err != nil {
		return nil, err
	}

	// Set initial size
	if err := setWinSize(master, cols, rows); err != nil {
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

	return &unixPTY{
		master: master,
	}, nil
}

// unixPTY implements PTY for Unix systems.
type unixPTY struct {
	master *os.File
}

func (p *unixPTY) File() *os.File {
	return p.master
}

func (p *unixPTY) Read(buf []byte) (int, error) {
	return p.master.Read(buf)
}

func (p *unixPTY) Write(data []byte) (int, error) {
	return p.master.Write(data)
}

func (p *unixPTY) Resize(cols, rows uint16) error {
	return setWinSize(p.master, cols, rows)
}

func (p *unixPTY) Close() error {
	return p.master.Close()
}

// openPTY opens a new PTY master/slave pair.
func openPTY() (*os.File, *os.File, error) {
	// Open /dev/ptmx to get master
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		return nil, nil, err
	}

	// Grant access to slave
	if err := grantPT(master); err != nil {
		master.Close()
		return nil, nil, err
	}

	// Unlock slave
	if err := unlockPT(master); err != nil {
		master.Close()
		return nil, nil, err
	}

	// Get slave path
	slavePath, err := ptsName(master)
	if err != nil {
		master.Close()
		return nil, nil, err
	}

	// Open slave
	slave, err := os.OpenFile(slavePath, os.O_RDWR, 0)
	if err != nil {
		master.Close()
		return nil, nil, err
	}

	return master, slave, nil
}

// grantPT grants access to the slave PTY.
func grantPT(master *os.File) error {
	// On modern Linux, this is a no-op
	return nil
}

// unlockPT unlocks the slave PTY.
func unlockPT(master *os.File) error {
	var unlock int32 = 0
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		master.Fd(),
		syscall.TIOCSPTLCK,
		uintptr(unsafe.Pointer(&unlock)),
	)
	if errno != 0 {
		return errno
	}
	return nil
}

// ptsName returns the path of the slave PTY.
func ptsName(master *os.File) (string, error) {
	var ptyno uint32
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		master.Fd(),
		syscall.TIOCGPTN,
		uintptr(unsafe.Pointer(&ptyno)),
	)
	if errno != 0 {
		return "", errno
	}
	return "/dev/pts/" + itoa(int(ptyno)), nil
}

// setWinSize sets the window size of the PTY.
func setWinSize(f *os.File, cols, rows uint16) error {
	ws := &winSize{
		Row: rows,
		Col: cols,
	}
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		f.Fd(),
		syscall.TIOCSWINSZ,
		uintptr(unsafe.Pointer(ws)),
	)
	if errno != 0 {
		return errno
	}
	return nil
}

// winSize is the structure used by TIOCSWINSZ.
type winSize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

// itoa converts int to string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
