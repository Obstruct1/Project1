package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/olekukonko/tablewriter"
)

func main() {
	// CLI args
	f, closeFile, err := openProcessingFile(os.Args...)
	if err != nil {
		log.Fatal(err)
	}
	defer closeFile()

	// Load and parse processes
	processes, err := loadProcesses(f)
	if err != nil {
		log.Fatal(err)
	}

	// First-come, first-serve scheduling
	FCFSSchedule(os.Stdout, "First-come, first-serve", processes)

	SJFSchedule(os.Stdout, "Shortest-job-first", processes)
	//
	//SJFPrioritySchedule(os.Stdout, "Priority", processes)
	//
	//RRSchedule(os.Stdout, "Round-robin", processes)
}

func openProcessingFile(args ...string) (*os.File, func(), error) {
	if len(args) != 2 {
		return nil, nil, fmt.Errorf("%w: must give a scheduling file to process", ErrInvalidArgs)
	}
	// Read in CSV process CSV file
	f, err := os.Open(args[1])
	if err != nil {
		return nil, nil, fmt.Errorf("%v: error opening scheduling file", err)
	}
	closeFn := func() {
		if err := f.Close(); err != nil {
			log.Fatalf("%v: error closing scheduling file", err)
		}
	}

	return f, closeFn, nil
}

type (
	Process struct {
		ProcessID     int64
		ArrivalTime   int64
		BurstDuration int64
		Priority      int64
	}
	TimeSlice struct {
		PID   int64
		Start int64
		Stop  int64
	}
)

//region Schedulers

// FCFSSchedule outputs a schedule of processes in a GANTT chart and a table of timing given:
// • an output writer
// • a title for the chart
// • a slice of processes
func FCFSSchedule(w io.Writer, title string, processes []Process) {
	var (
		serviceTime     int64
		totalWait       float64
		turnaround float64
		lastCompletion  float64
		waitingTime     int64
		schedule        = make([][]string, len(processes))
		gantt           = make([]TimeSlice, 0)
	)
	for i := range processes {
		if processes[i].ArrivalTime > 0 {
			waitingTime = serviceTime - processes[i].ArrivalTime
		}
		totalWait += float64(waitingTime)

		start := waitingTime + processes[i].ArrivalTime

		turnaround := processes[i].BurstDuration + waitingTime
		turnaround += float64(turnaround)

		completion := processes[i].BurstDuration + processes[i].ArrivalTime + waitingTime
		lastCompletion = float64(completion)

		schedule[i] = []string{
			fmt.Sprint(processes[i].ProcessID),
			fmt.Sprint(processes[i].Priority),
			fmt.Sprint(processes[i].BurstDuration),
			fmt.Sprint(processes[i].ArrivalTime),
			fmt.Sprint(waitingTime),
			fmt.Sprint(turnaround),
			fmt.Sprint(completion),
		}
		serviceTime += processes[i].BurstDuration

		gantt = append(gantt, TimeSlice{
			PID:   processes[i].ProcessID,
			Start: start,
			Stop:  serviceTime,
		})
	}

	count := float64(len(processes))
	aveWait := totalWait / count
	aveTurnaround := turnaround / count
	aveThroughput := count / lastCompletion

	outputTitle(w, title)
	outputGantt(w, gantt)
	outputSchedule(w, schedule, aveWait, aveTurnaround, aveThroughput)
}

func SJFSchedule(w io.Writer, title string, processes []Process) {
	// Sort processes by arrival time initially
	sort.Slice(processes, func(i, j int) bool {
		return processes[i].ArrivalTime < processes[j].ArrivalTime
	})

	var (
		currentTime     int64
		turnaround int64
		totalWait       int64
		completed       int
	)

	// Implementing a min heap to always get the process with the shortest burst time
	h := &IntHeap{}
	heap.Init(h)

	for completed < len(processes) || h.Len() > 0 {
		for _, p := range processes {
			if p.ArrivalTime <= currentTime && p.BurstDuration > 0 {
				heap.Push(h, p)
			}
		}

		if h.Len() > 0 {
			p := heap.Pop(h).(Process)
			p.BurstDuration--
			currentTime++

			if p.BurstDuration == 0 {
				completed++
				turnaround += currentTime - p.ArrivalTime
				totalWait += currentTime - p.ArrivalTime - p.BurstDuration
			} else {
				heap.Push(h, p)
			}
		} else {
			currentTime++
		}
	}

	// Calculating averages
	avgTurnaround := float64(turnaround) / float64(len(processes))
	avgWait := float64(totalWait) / float64(len(processes))
	throughput := float64(len(processes)) / float64(currentTime)

	// Output the results
	fmt.Printf("Average Turnaround Time: %.2f\n", avgTurnaround)
	fmt.Printf("Average Waiting Time: %.2f\n", avgWait)
	fmt.Printf("Throughput: %.2f\n", throughput)
}
//
func SJFSchedule(w io.Writer, title string, processes []Process) {
	var (
		totalWait       float64
		totalTurnaround float64
		schedule        = make([][]string, 0)
		gantt           = make([]TimeSlice, 0)
		currentTime     int64
	)

	// Create a priority queue with a custom Less function to consider both priority and burst time
	h := &PriorityHeap{}
	heap.Init(h)

	for len(*h) > 0 || len(processes) > 0 {
		for i := 0; i < len(processes); {
			if processes[i].ArrivalTime <= currentTime && processes[i].BurstDuration > 0 {
				heap.Push(h, processes[i])
				processes = append(processes[:i], processes[i+1:]...)
				continue
			}
			i++
		}

		if h.Len() > 0 {
			p := heap.Pop(h).(Process)
			startTime := currentTime
			currentTime += p.BurstDuration
			waitTime := startTime - p.ArrivalTime
			turnaroundTime := currentTime - p.ArrivalTime

			totalWait += float64(waitTime)
			totalTurnaround += float64(turnaroundTime)

			schedule = append(schedule, []string{
				fmt.Sprint(p.ProcessID),
				fmt.Sprint(p.Priority),
				fmt.Sprint(p.BurstDuration + waitTime), // original burst time
				fmt.Sprint(p.ArrivalTime),
				fmt.Sprint(waitTime),
				fmt.Sprint(turnaroundTime),
				fmt.Sprint(currentTime),
			})

			gantt = append(gantt, TimeSlice{
				PID:   p.ProcessID,
				Start: startTime,
				Stop:  currentTime,
			})
		} else {
			currentTime++
		}
	}

	// Calculating averages
	avgWait := totalWait / float64(len(schedule))
	avgTurnaround := totalTurnaround / float64(len(schedule))
	throughput := float64(len(schedule)) / float64(currentTime)

	// Output the results in a similar format to your original FCFSSchedule function
	outputTitle(w, title)
	outputGantt(w, gantt)
	outputSchedule(w, schedule, avgWait, avgTurnaround, throughput)
}
//
func RRSchedule(w io.Writer, title string, processes []Process) {
	timeQuantum := 2
	var (
		totalWait       int64
		turnaround int64
		currentTime     int64
	)

	// Using a queue to handle processes
	queue := make([]Process, 0)

	for len(queue) > 0 || len(processes) > 0 {
		for i := 0; i < len(processes); {
			if processes[i].ArrivalTime <= currentTime {
				queue = append(queue, processes[i])
				// Remove the added process from the original slice
				processes = append(processes[:i], processes[i+1:]...)
				continue
			}
			i++
		}

		if len(queue) > 0 {
			p := queue[0]
			queue = queue[1:]

			if p.BurstDuration > int64(timeQuantum) {
				currentTime += int64(timeQuantum)
				p.BurstDuration -= int64(timeQuantum)
				queue = append(queue, p) // Add back to the queue if not finished
			} else {
				currentTime += p.BurstDuration
				turnaround += currentTime - p.ArrivalTime
				totalWait += currentTime - p.ArrivalTime - p.BurstDuration
			}
		} else {
			currentTime++
		}
	}

	// Calculating averages
	avgTurnaround := float64(turnaround) / float64(len(processes))
	avgWait := float64(totalWait) / float64(len(processes))
	throughput := float64(len(processes)) / float64(currentTime)

	// Output the results
	fmt.Printf("Average Turnaround Time: %.2f\n", avgTurnaround)
	fmt.Printf("Average Waiting Time: %.2f\n", avgWait)
	fmt.Printf("Throughput: %.2f\n", throughput)
}

//endregion

//region Output helpers

func outputTitle(w io.Writer, title string) {
	_, _ = fmt.Fprintln(w, strings.Repeat("-", len(title)*2))
	_, _ = fmt.Fprintln(w, strings.Repeat(" ", len(title)/2), title)
	_, _ = fmt.Fprintln(w, strings.Repeat("-", len(title)*2))
}

func outputGantt(w io.Writer, gantt []TimeSlice) {
	_, _ = fmt.Fprintln(w, "Gantt schedule")
	_, _ = fmt.Fprint(w, "|")
	for i := range gantt {
		pid := fmt.Sprint(gantt[i].PID)
		padding := strings.Repeat(" ", (8-len(pid))/2)
		_, _ = fmt.Fprint(w, padding, pid, padding, "|")
	}
	_, _ = fmt.Fprintln(w)
	for i := range gantt {
		_, _ = fmt.Fprint(w, fmt.Sprint(gantt[i].Start), "\t")
		if len(gantt)-1 == i {
			_, _ = fmt.Fprint(w, fmt.Sprint(gantt[i].Stop))
		}
	}
	_, _ = fmt.Fprintf(w, "\n\n")
}

func outputSchedule(w io.Writer, rows [][]string, wait, turnaround, throughput float64) {
	_, _ = fmt.Fprintln(w, "Schedule table")
	table := tablewriter.NewWriter(w)
	table.SetHeader([]string{"ID", "Priority", "Burst", "Arrival", "Wait", "Turnaround", "Exit"})
	table.AppendBulk(rows)
	table.SetFooter([]string{"", "", "", "",
		fmt.Sprintf("Average\n%.2f", wait),
		fmt.Sprintf("Average\n%.2f", turnaround),
		fmt.Sprintf("Throughput\n%.2f/t", throughput)})
	table.Render()
}

//endregion

//region Loading processes.

var ErrInvalidArgs = errors.New("invalid args")

func loadProcesses(r io.Reader) ([]Process, error) {
	rows, err := csv.NewReader(r).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("%w: reading CSV", err)
	}

	processes := make([]Process, len(rows))
	for i := range rows {
		processes[i].ProcessID = mustStrToInt(rows[i][0])
		processes[i].BurstDuration = mustStrToInt(rows[i][1])
		processes[i].ArrivalTime = mustStrToInt(rows[i][2])
		if len(rows[i]) == 4 {
			processes[i].Priority = mustStrToInt(rows[i][3])
		}
	}

	return processes, nil
}

func mustStrToInt(s string) int64 {
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	return i
}

//endregion
