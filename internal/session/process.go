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

func IsWaitingForInput(shellPID int) bool {
	return isWaitingForInput(shellPID)
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

// ProcInfo is one process's foreground-group and wait-channel state, as gathered in bulk by
// LoadProcTable instead of the two separate `ps -p <pid>` subprocess calls isWaitingForInput
// used to make per session (on Linux these two facts come from free /proc reads instead, so
// this table is only built/used on other platforms — see NeedsProcTable).
type ProcInfo struct {
	TPGID int
	WChan string
}

// NeedsProcTable reports whether this platform requires LoadProcTable at all: Linux's
// isWaitingForInput already reads /proc directly with zero subprocess cost, so building and
// looking up a shared ps table there would be pure overhead with no benefit.
func NeedsProcTable() bool {
	return runtime.GOOS != "linux"
}

// LoadProcTable runs a single `ps -A` for every process on the machine, replacing what used to
// be two separate `ps -p <pid>` subprocess calls per session (tpgid, then wchan) — with many
// sessions those add up fast, while this one call's cost is paid exactly once per poll round
// regardless of session count.
func LoadProcTable() (map[int]ProcInfo, error) {
	out, err := exec.Command("ps", "-A", "-o", "pid=,tpgid=,wchan=").Output()
	if err != nil {
		return nil, err
	}
	table := make(map[int]ProcInfo)
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		tpgid, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		wchan := ""
		if len(fields) >= 3 {
			wchan = fields[2]
		}
		table[pid] = ProcInfo{TPGID: tpgid, WChan: wchan}
	}
	return table, nil
}

// isWaitingForInputFromTable is the table-driven equivalent of isWaitingForInput, looking up
// both facts from a pre-fetched ProcInfo map instead of spawning a `ps` subprocess per call.
func isWaitingForInputFromTable(shellPID int, table map[int]ProcInfo) bool {
	info, ok := table[shellPID]
	if !ok || info.TPGID <= 0 || info.TPGID == shellPID {
		return false
	}
	fg, ok := table[info.TPGID]
	if !ok {
		return false
	}
	return inputWchans[fg.WChan]
}
