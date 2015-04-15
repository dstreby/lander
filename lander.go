package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type SysInfo struct {
	CPUUsage     string
	Hostname     string
	Uptime       time.Duration
	Load1        string
	Load5        string
	Load15       string
	Procs        uint64
	TotalRam     uint64
	FreeRam      uint64
	SharedRam    uint64
	BufferRam    uint64
	TotalSwp     uint64
	FreeSwp      uint64
	TotalHighRam uint64
	FreeHighRam  uint64
	mu           sync.Mutex
}

var sysInfo SysInfo

func getHostname() {
	hostname, err := os.Hostname()
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}
	sysInfo.Hostname = hostname
}

func getSysinfo() {
	si := &syscall.Sysinfo_t{}

	err := syscall.Sysinfo(si)
	if err != nil {
		fmt.Println("Error: ", err)
	}

	scale := 65536.0
	unit := uint64(si.Unit) * 1024 * 1024 // MiB

	defer sysInfo.mu.Unlock()
	sysInfo.mu.Lock()

	sysInfo.Uptime = time.Duration(si.Uptime) * time.Second
	sysInfo.Load1 = fmt.Sprintf("%2.2f", (float64(si.Loads[0]) / scale))
	sysInfo.Load5 = fmt.Sprintf("%2.2f", (float64(si.Loads[1]) / scale))
	sysInfo.Load15 = fmt.Sprintf("%2.2f", (float64(si.Loads[2]) / scale))
	sysInfo.Procs = uint64(si.Procs)
	sysInfo.TotalRam = uint64(si.Totalram) / unit
	sysInfo.FreeRam = uint64(si.Freeram) / unit
	sysInfo.BufferRam = uint64(si.Bufferram) / unit
	sysInfo.TotalSwp = uint64(si.Totalswap) / unit
	sysInfo.FreeSwp = uint64(si.Freeswap) / unit
	sysInfo.TotalHighRam = uint64(si.Totalhigh) / unit
	sysInfo.FreeHighRam = uint64(si.Freehigh) / unit

}

func getCPUTime() (total, idle uint64) {
	proc_stat, err := ioutil.ReadFile("/proc/stat")
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}
	lines := strings.Split(string(proc_stat), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if fields[0] == "cpu" {
			numFields := len(fields)
			for i := 1; i < numFields; i++ {
				val, err := strconv.ParseUint(fields[i], 10, 64)
				if err != nil {
					fmt.Println("Error: ", i, fields[i], err)
				}
				total += val
				if i == 4 || i == 5 {
					idle += val
				}
			}
			return
		}
	}
	return
}

func getCPUUsage(delay int) (cpuPercent float64) {
	totalPrev, idlePrev := getCPUTime()
	time.Sleep(time.Duration(delay) * time.Millisecond)
	totalCur, idleCur := getCPUTime()
	cpuPercent = (math.Float64frombits(((totalCur - totalPrev) - (idleCur - idlePrev))) / math.Float64frombits((totalCur - totalPrev))) * 100
	return
}

func worker() {
	for {
		getSysinfo()
		usage := getCPUUsage(3000)
		sysInfo.mu.Lock()
		sysInfo.CPUUsage = fmt.Sprintf("%.2f", usage)
		sysInfo.mu.Unlock()
	}
}

func httpHandler(w http.ResponseWriter, r *http.Request) {
	fp := path.Join("templates", "index.html")

	tmpl, err := template.ParseFiles(fp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, sysInfo)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func main() {
	getHostname()
	for w := 1; w <= 4; w++ {
		go worker()
	}
	http.HandleFunc("/", httpHandler)
	http.ListenAndServe(":5000", nil)
}
