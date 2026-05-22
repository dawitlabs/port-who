package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const version = "0.1.0"

const (
	reset  = "\033[0m"
	bold   = "\033[1m"
	dim    = "\033[2m"
	red    = "\033[31m"
	green  = "\033[32m"
	yellow = "\033[33m"
	cyan   = "\033[36m"
)

type procInfo struct {
	pid    int
	name   string
	cmd    string
	uptime time.Duration
}

func parseHexPort(localAddr string) (int, error) {
	// format: "0F02000A:1F90" — port is after the colon
	idx := strings.LastIndex(localAddr, ":")
	if idx < 0 {
		return 0, fmt.Errorf("bad addr")
	}
	p, err := strconv.ParseInt(localAddr[idx+1:], 16, 32)
	return int(p), err
}

func scanTCP(path string, port int) []int64 {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var inodes []int64
	sc := bufio.NewScanner(f)
	sc.Scan() // skip header
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 10 {
			continue
		}
		p, err := parseHexPort(fields[1])
		if err != nil || p != port {
			continue
		}
		if fields[3] != "0A" { // 0A = TCP_LISTEN
			continue
		}
		inode, err := strconv.ParseInt(fields[9], 10, 64)
		if err != nil {
			continue
		}
		inodes = append(inodes, inode)
	}
	return inodes
}

func inodeToPID(inode int64) int {
	target := fmt.Sprintf("socket:[%d]", inode)
	entries, _ := filepath.Glob("/proc/*/fd/*")
	for _, entry := range entries {
		link, err := os.Readlink(entry)
		if err != nil || link != target {
			continue
		}
		parts := strings.Split(entry, "/")
		if len(parts) < 3 {
			continue
		}
		pid, err := strconv.Atoi(parts[2])
		if err != nil {
			continue
		}
		return pid
	}
	return -1
}

func procUptime(pid int) time.Duration {
	uptimeData, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	sysUp, err := strconv.ParseFloat(strings.Fields(string(uptimeData))[0], 64)
	if err != nil {
		return 0
	}

	statData, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0
	}
	// skip past the comm field "(name)" which may contain spaces
	s := string(statData)
	rp := strings.LastIndex(s, ")")
	if rp < 0 {
		return 0
	}
	fields := strings.Fields(s[rp+1:])
	if len(fields) < 20 {
		return 0
	}
	startTicks, err := strconv.ParseFloat(fields[19], 64)
	if err != nil {
		return 0
	}
	upSec := sysUp - startTicks/100.0 // HZ=100
	if upSec < 0 {
		upSec = 0
	}
	return time.Duration(upSec * float64(time.Second))
}

func getProc(pid int) procInfo {
	info := procInfo{pid: pid}

	if b, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid)); err == nil {
		info.name = strings.TrimSpace(string(b))
	}
	if b, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid)); err == nil {
		parts := strings.Split(string(b), "\x00")
		var args []string
		for _, p := range parts {
			if p != "" {
				args = append(args, p)
			}
		}
		info.cmd = strings.Join(args, " ")
		if len(info.cmd) > 100 {
			info.cmd = info.cmd[:100] + "…"
		}
	}
	info.uptime = procUptime(pid)
	return info
}

func fmtDur(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	default:
		return fmt.Sprintf("%dd %dh", int(d.Hours())/24, int(d.Hours())%24)
	}
}

func main() {
	kill := flag.Bool("kill", false, "send SIGTERM to the process using this port")
	flag.Parse()

	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: port-who [--kill] <port>")
		os.Exit(1)
	}
	port, err := strconv.Atoi(flag.Arg(0))
	if err != nil || port < 1 || port > 65535 {
		fmt.Fprintf(os.Stderr, "invalid port: %s\n", flag.Arg(0))
		os.Exit(1)
	}

	var inodes []int64
	inodes = append(inodes, scanTCP("/proc/net/tcp", port)...)
	inodes = append(inodes, scanTCP("/proc/net/tcp6", port)...)

	if len(inodes) == 0 {
		fmt.Printf("\n  port %s%d%s is %sfree%s\n\n", bold, port, reset, green, reset)
		return
	}

	seen := map[int]bool{}
	var procs []procInfo
	for _, inode := range inodes {
		pid := inodeToPID(inode)
		if pid < 0 || seen[pid] {
			continue
		}
		seen[pid] = true
		procs = append(procs, getProc(pid))
	}

	if len(procs) == 0 {
		fmt.Printf("\n  port %s%d%s is in use %s(try sudo to see the process)%s\n\n", bold, port, reset, dim, reset)
		return
	}

	pad := strings.Repeat(" ", max(0, 20-len(strconv.Itoa(port))))
	fmt.Printf("\n%s╔══════════════════════════════════════════╗%s\n", cyan, reset)
	fmt.Printf("%s║%s  %sport-who%s  ·  :%s%d%s%s  %s║%s\n", cyan, reset, bold, reset, yellow, port, reset, pad, cyan, reset)
	fmt.Printf("%s╚══════════════════════════════════════════╝%s\n", cyan, reset)

	for _, p := range procs {
		fmt.Printf("\n  %s%s%s  %sPID %d%s\n", bold, p.name, reset, dim, p.pid, reset)
		if p.cmd != "" {
			fmt.Printf("  %s%s%s\n", dim, p.cmd, reset)
		}
		if p.uptime > 0 {
			fmt.Printf("  uptime  %s%s%s\n", yellow, fmtDur(p.uptime), reset)
		}
	}

	fmt.Println()
	if *kill {
		for _, p := range procs {
			fmt.Printf("  killing %s%s%s (PID %d) … ", bold, p.name, reset, p.pid)
			if err := syscall.Kill(p.pid, syscall.SIGTERM); err != nil {
				fmt.Printf("%s✗ %s%s\n", red, err, reset)
			} else {
				fmt.Printf("%s✓%s\n", green, reset)
			}
		}
		fmt.Println()
	} else {
		fmt.Printf("  %sport-who --kill %d%s  to terminate\n\n", dim, port, reset)
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
