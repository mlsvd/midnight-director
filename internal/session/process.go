package session

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// inputWchans are kernel wait-channel values that indicate a process is
// blocked reading from a terminal (Linux and macOS).
var inputWchans = map[string]bool{
	"n_tty_read":            true, // Linux: classic terminal read
	"wait_woken":            true, // Linux: newer kernel terminal read
	"ep_poll":               true, // Linux: epoll wait (async I/O apps, e.g. Claude CLI)
	"poll_schedule_timeout": true, // Linux: poll/select on terminal fd
	"ttyin":                 true, // macOS: waiting for terminal input
}

func isWaitingForInput(shellPID int) bool {
	tpgid, err := getForegroundPGID(shellPID)
	if err != nil || tpgid <= 0 || tpgid == shellPID {
		return false
	}
	wchan, err := getWchan(tpgid)
	if err != nil {
		return false
	}
	return inputWchans[wchan]
}

func getForegroundPGID(shellPID int) (int, error) {
	if runtime.GOOS == "linux" {
		return getForegroundPGIDLinux(shellPID)
	}
	return getForegroundPGIDPS(shellPID)
}

func getForegroundPGIDLinux(shellPID int) (int, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", shellPID))
	if err != nil {
		return 0, err
	}
	// format: pid (comm) state ppid pgrp session tty_nr tpgid ...
	// comm may contain spaces/parens, so find the last ')' to skip it
	idx := bytes.LastIndexByte(data, ')')
	if idx < 0 {
		return 0, fmt.Errorf("malformed /proc/stat")
	}
	fields := strings.Fields(string(data[idx+1:]))
	// [0]=state [1]=ppid [2]=pgrp [3]=session [4]=tty_nr [5]=tpgid
	if len(fields) < 6 {
		return 0, fmt.Errorf("not enough fields in /proc/stat")
	}
	tpgid, err := strconv.Atoi(fields[5])
	if err != nil || tpgid < 0 {
		return 0, fmt.Errorf("invalid tpgid")
	}
	return tpgid, nil
}

func getForegroundPGIDPS(shellPID int) (int, error) {
	out, err := exec.Command("ps", "-p", strconv.Itoa(shellPID), "-o", "tpgid=").Output()
	if err != nil {
		return 0, err
	}
	tpgid, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil || tpgid < 0 {
		return 0, fmt.Errorf("invalid tpgid")
	}
	return tpgid, nil
}

func getWchan(pid int) (string, error) {
	if runtime.GOOS == "linux" {
		data, err := os.ReadFile(fmt.Sprintf("/proc/%d/wchan", pid))
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(data)), nil
	}
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "wchan=").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
