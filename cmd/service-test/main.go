package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func main() {
	fmt.Println("Testing service management functions...")
	
	// Test isServiceRunning function
	running := isServiceRunning()
	fmt.Printf("Service running: %v\n", running)
	
	if len(os.Args) > 1 && os.Args[1] == "start" {
		fmt.Println("Starting service...")
		startService()
	}
	
	if len(os.Args) > 1 && os.Args[1] == "stop" {
		fmt.Println("Stopping service...")
		stopService()
	}
}

func isServiceRunning() bool {
	// Check if simpledb-mcp process is running
	cmd := exec.Command("pgrep", "-f", "simpledb-mcp")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	
	// Filter out our own CLI process
	pids := strings.Fields(string(output))
	myPid := fmt.Sprintf("%d", os.Getpid())
	
	for _, pid := range pids {
		if pid != myPid {
			// Check if this PID is actually the server by looking at command line
			cmdlineCmd := exec.Command("ps", "-p", pid, "-o", "args=")
			cmdOutput, err := cmdlineCmd.Output()
			if err != nil {
				continue
			}
			
			cmdline := string(cmdOutput)
			// Look for the server binary, not the CLI
			if strings.Contains(cmdline, "simpledb-mcp") && !strings.Contains(cmdline, "simpledb-cli") {
				fmt.Printf("Found running service: PID %s, Command: %s\n", pid, strings.TrimSpace(cmdline))
				return true
			}
		}
	}
	
	return false
}

func startService() {
	// Start the service in background
	cmd := exec.Command("nohup", "./bin/simpledb-mcp", "&")
	
	if err := cmd.Start(); err != nil {
		fmt.Printf("Failed to start service: %v\n", err)
		return
	}
	
	fmt.Println("Service start command executed")
}

func stopService() {
	// Find and kill the service process
	cmd := exec.Command("pkill", "-f", "simpledb-mcp")
	if err := cmd.Run(); err != nil {
		fmt.Printf("pkill failed: %v\n", err)
		
		// Try alternative approach with pgrep + kill
		pgrepCmd := exec.Command("pgrep", "-f", "simpledb-mcp")
		output, err := pgrepCmd.Output()
		if err != nil {
			fmt.Printf("Failed to find running service process: %v\n", err)
			return
		}
		
		pids := strings.Fields(string(output))
		fmt.Printf("Found PIDs to kill: %v\n", pids)
		for _, pid := range pids {
			killCmd := exec.Command("kill", pid)
			if err := killCmd.Run(); err != nil {
				fmt.Printf("Failed to kill PID %s: %v\n", pid, err)
			} else {
				fmt.Printf("Killed PID %s\n", pid)
			}
		}
	} else {
		fmt.Println("Service stopped with pkill")
	}
}