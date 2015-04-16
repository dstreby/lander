package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"math"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type SysInfo struct {
	Hostname     string
	Sysname      string
	Release      string
	Version      string
	Machine      string
	Domainname   string
	CPUUsage     string
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

var SI SysInfo

func utsToStr(intarr *[65]int8) string {
	var str string
	for _, val := range *intarr {
		str += string(int(val))
	}

	return strings.Trim(str, "\000")
}

func getUname() {
	uts := &syscall.Utsname{}

	if err := syscall.Uname(uts); err != nil {
		fmt.Println("Error: ", err)
	}

	SI.Sysname = utsToStr(&uts.Sysname)
	SI.Hostname = utsToStr(&uts.Nodename)
	SI.Release = utsToStr(&uts.Release)
	SI.Version = utsToStr(&uts.Version)
	SI.Machine = utsToStr(&uts.Machine)
	SI.Domainname = utsToStr(&uts.Domainname)
}

func getSysinfo() {
	si := &syscall.Sysinfo_t{}

	err := syscall.Sysinfo(si)
	if err != nil {
		fmt.Println("Error: ", err)
	}

	scale := 65536.0
	unit := uint64(si.Unit) * 1024 * 1024 // MiB

	defer SI.mu.Unlock()
	SI.mu.Lock()

	SI.Uptime = time.Duration(si.Uptime) * time.Second
	SI.Load1 = fmt.Sprintf("%2.2f", (float64(si.Loads[0]) / scale))
	SI.Load5 = fmt.Sprintf("%2.2f", (float64(si.Loads[1]) / scale))
	SI.Load15 = fmt.Sprintf("%2.2f", (float64(si.Loads[2]) / scale))
	SI.Procs = uint64(si.Procs)
	SI.TotalRam = uint64(si.Totalram) / unit
	SI.FreeRam = uint64(si.Freeram) / unit
	SI.BufferRam = uint64(si.Bufferram) / unit
	SI.TotalSwp = uint64(si.Totalswap) / unit
	SI.FreeSwp = uint64(si.Freeswap) / unit
	SI.TotalHighRam = uint64(si.Totalhigh) / unit
	SI.FreeHighRam = uint64(si.Freehigh) / unit

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
		SI.mu.Lock()
		SI.CPUUsage = fmt.Sprintf("%.2f", usage)
		SI.mu.Unlock()
	}
}

func httpHandler(w http.ResponseWriter, r *http.Request) {
	fp := path.Join("templates", "index.html")

	tmpl, err := template.ParseFiles(fp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, SI)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func main() {
	getUname()

	for w := 1; w <= 4; w++ {
		go worker()
	}

	fs := http.FileServer(http.Dir("assets"))
	http.Handle("/assets/", http.StripPrefix("/assets/", fs))
	http.HandleFunc("/", httpHandler)
	http.ListenAndServe(":5000", nil)
}
