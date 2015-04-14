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
	"time"
)

type SysInfo struct {
	CPUUsage string
	Hostname string
}

// This is horrible and needs a proper implementation
var sysInfo SysInfo

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
		sysInfo.CPUUsage = fmt.Sprintf("%.2f", getCPUUsage(3000))
	}
}

func httpHandler(w http.ResponseWriter, r *http.Request) {
	fp := path.Join("templates", "index.html")
	tmpl, err := template.ParseFiles(fp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, sysInfo); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func main() {
	go worker()
	sysInfo.Hostname, _ = os.Hostname()
	http.HandleFunc("/", httpHandler)
	http.ListenAndServe(":5000", nil)
}
