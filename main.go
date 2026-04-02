package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"
)

const version = "0.1.0"

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	switch args[0] {
	case "--help", "-h":
		printUsage()
		return
	case "--version", "-v":
		fmt.Println("whoisusing", version)
		return
	case "--all", "-a":
		entries, err := listAllPorts()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}
		if len(entries) == 0 {
			fmt.Println("No listening ports found.")
			return
		}
		printEntries(entries)
		return
	}

	port, err := strconv.Atoi(args[0])
	if err != nil || port < 1 || port > 65535 {
		fmt.Fprintf(os.Stderr, "Error: %q is not a valid port (1-65535)\n", args[0])
		os.Exit(1)
	}

	entries, err := findByPort(port)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	if len(entries) == 0 {
		fmt.Printf("Port %d is free — nothing is using it.\n", port)
		return
	}

	printEntries(entries)
}

func printUsage() {
	fmt.Println(`whoisusing — find what's using a port

Usage:
  whoisusing <port>       Show process using a specific port
  whoisusing --all        List all listening ports
  whoisusing --help       Show this help
  whoisusing --version    Show version

Examples:
  whoisusing 8080
  whoisusing 3000
  whoisusing --all`)
}

// Entry holds info about a process using a port.
type Entry struct {
	Proto   string
	Local   string
	Port    int
	PID     int
	Process string
}

func findByPort(port int) ([]Entry, error) {
	switch runtime.GOOS {
	case "windows":
		return findByPortWindows(port)
	case "linux", "darwin":
		return findByPortUnix(port)
	default:
		return nil, fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

func listAllPorts() ([]Entry, error) {
	switch runtime.GOOS {
	case "windows":
		return listAllPortsWindows()
	case "linux", "darwin":
		return listAllPortsUnix()
	default:
		return nil, fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

func printEntries(entries []Entry) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PROTO\tPORT\tPID\tPROCESS")
	for _, e := range entries {
		fmt.Fprintf(w, "%s\t%d\t%d\t%s\n", e.Proto, e.Port, e.PID, e.Process)
	}
	w.Flush()
}

// --- Windows ---

func scanWindows(filterPort int) ([]Entry, error) {
	type result struct {
		entries []Entry
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	var entries []Entry

	for _, proto := range []string{"TCP", "TCPv6", "UDP", "UDPv6"} {
		wg.Add(1)
		go func(proto string) {
			defer wg.Done()
			out, err := exec.Command("cmd", "/C", "netstat -ano -p "+proto).CombinedOutput()
			if err != nil {
				return
			}
			label := strings.TrimSuffix(proto, "v6")
			parsed := parseNetstatWindows(string(out), label, filterPort)
			mu.Lock()
			entries = append(entries, parsed...)
			mu.Unlock()
		}(proto)
	}
	wg.Wait()

	if len(entries) == 0 {
		return nil, nil
	}

	entries = resolveProcessNames(entries)
	return dedup(entries), nil
}

func findByPortWindows(port int) ([]Entry, error) {
	return scanWindows(port)
}

func listAllPortsWindows() ([]Entry, error) {
	return scanWindows(-1)
}

func parseNetstatWindows(output, proto string, filterPort int) []Entry {
	var entries []Entry
	seen := make(map[string]bool)

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		// Only LISTENING or UDP lines
		if proto == "TCP" && (len(fields) < 5 || fields[3] != "LISTENING") {
			continue
		}

		local := fields[1]
		pidStr := fields[len(fields)-1]

		port := extractPort(local)
		if port == -1 {
			continue
		}
		if filterPort != -1 && port != filterPort {
			continue
		}

		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}

		key := fmt.Sprintf("%s:%d:%d", proto, port, pid)
		if seen[key] {
			continue
		}
		seen[key] = true

		entries = append(entries, Entry{
			Proto: proto,
			Local: local,
			Port:  port,
			PID:   pid,
		})
	}
	return entries
}

func resolveProcessNames(entries []Entry) []Entry {
	if runtime.GOOS == "windows" {
		return resolveWindows(entries)
	}
	return resolveUnix(entries)
}

func resolveWindows(entries []Entry) []Entry {
	// One tasklist call for ALL processes, build a PID→name map
	out, err := exec.Command("tasklist", "/FO", "CSV", "/NH").CombinedOutput()
	if err != nil {
		for i := range entries {
			entries[i].Process = "(unknown)"
		}
		return entries
	}

	pidMap := make(map[int]string)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		parts := strings.Split(line, ",")
		if len(parts) < 2 {
			continue
		}
		name := strings.Trim(parts[0], "\"")
		pid, err := strconv.Atoi(strings.Trim(parts[1], "\""))
		if err != nil {
			continue
		}
		pidMap[pid] = name
	}

	for i, e := range entries {
		if name, ok := pidMap[e.PID]; ok {
			entries[i].Process = name
		} else {
			entries[i].Process = "(unknown)"
		}
	}
	return entries
}

func resolveUnix(entries []Entry) []Entry {
	for i, e := range entries {
		out, err := exec.Command("ps", "-p", strconv.Itoa(e.PID), "-o", "comm=").CombinedOutput()
		if err != nil {
			entries[i].Process = "(unknown)"
		} else {
			entries[i].Process = strings.TrimSpace(string(out))
		}
	}
	return entries
}

// --- Unix (Linux/macOS) ---

func findByPortUnix(port int) ([]Entry, error) {
	out, err := exec.Command("lsof", "-i", fmt.Sprintf(":%d", port), "-P", "-n", "-sTCP:LISTEN").CombinedOutput()
	if err != nil {
		// lsof returns exit 1 when nothing found
		if len(out) == 0 {
			return nil, nil
		}
	}

	return parseLsof(string(out)), nil
}

func listAllPortsUnix() ([]Entry, error) {
	out, err := exec.Command("lsof", "-i", "-P", "-n", "-sTCP:LISTEN").CombinedOutput()
	if err != nil {
		if len(out) == 0 {
			return nil, nil
		}
	}

	return dedup(parseLsof(string(out))), nil
}

func parseLsof(output string) []Entry {
	var entries []Entry
	lines := strings.Split(output, "\n")

	for _, line := range lines[1:] { // skip header
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}

		name := fields[0]
		pid, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}

		// field 8 is like "127.0.0.1:8080" or "*:8080"
		addr := fields[8]
		port := extractPort(addr)
		if port == -1 {
			continue
		}

		entries = append(entries, Entry{
			Proto:   "TCP",
			Local:   addr,
			Port:    port,
			PID:     pid,
			Process: name,
		})
	}
	return entries
}

// --- Helpers ---

func extractPort(address string) int {
	idx := strings.LastIndex(address, ":")
	if idx == -1 {
		return -1
	}
	p, err := strconv.Atoi(address[idx+1:])
	if err != nil {
		return -1
	}
	return p
}

func dedup(entries []Entry) []Entry {
	seen := make(map[string]bool)
	var result []Entry
	for _, e := range entries {
		key := fmt.Sprintf("%s:%d:%d", e.Proto, e.Port, e.PID)
		if !seen[key] {
			seen[key] = true
			result = append(result, e)
		}
	}
	return result
}
