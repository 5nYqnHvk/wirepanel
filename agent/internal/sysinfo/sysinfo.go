package sysinfo

import (
	"bufio"
	"encoding/json"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/wirepanel/wirepanel/shared/proto"
)

func Collect() (json.RawMessage, error) {
	hostname, _ := os.Hostname()
	info := proto.SystemInfoResult{
		Hostname: hostname,
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		CPUCount: runtime.NumCPU(),
	}
	if data, err := os.ReadFile("/proc/version"); err == nil {
		info.Kernel = strings.TrimSpace(string(data))
	}
	info.Distro = readDistro()
	if si := readSysinfo(); si != nil {
		info.Uptime = si.Uptime
		info.LoadAvg1 = float64(si.Loads[0]) / 65536.0
		info.LoadAvg5 = float64(si.Loads[1]) / 65536.0
		info.LoadAvg15 = float64(si.Loads[2]) / 65536.0
		info.MemTotalKB = int64(si.Totalram) * int64(si.Unit) / 1024
		info.MemFreeKB = int64(si.Freeram) * int64(si.Unit) / 1024
	}
	if v := readMeminfoKey("MemAvailable"); v > 0 {
		info.MemAvailKB = v
	}
	return json.Marshal(info)
}

func readDistro() string {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return ""
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			v := strings.TrimPrefix(line, "PRETTY_NAME=")
			return strings.Trim(v, `"`)
		}
	}
	return ""
}

func readSysinfo() *syscall.Sysinfo_t {
	var si syscall.Sysinfo_t
	if err := syscall.Sysinfo(&si); err != nil {
		return nil
	}
	return &si
}

func readMeminfoKey(key string) int64 {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()
		if strings.HasPrefix(line, key+":") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				v, _ := strconv.ParseInt(fields[1], 10, 64)
				return v
			}
		}
	}
	return 0
}
