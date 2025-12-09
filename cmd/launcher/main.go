package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

// --- CONFIGURATION ---
const (
	NodeCount     = 150	              // How many nodes to launch?
	StartHTTPPort = 8000             // Node 0 = 8000, Node 1 = 8001...
	StartUDPPort  = 9000             // Node 0 = 9000, Node 1 = 9001...
	BootstrapAddr = "127.0.0.1:9000" // Address of Node 0 (Genesis)
	ProjectRoot   = "../../"         // Path to the main.go file from here
)

var cmds []*exec.Cmd

func main() {
	// 1. Get Absolute Path to main.go (so we can run it from anywhere)
	absRoot, _ := filepath.Abs(ProjectRoot)
	mainGoPath := filepath.Join(absRoot, "main.go")
	frontendPath := filepath.Join(absRoot, "frontend")

	fmt.Printf("[Launcher] Target main.go: %s\n", mainGoPath)

	// 2. Clean up previous run
	os.RemoveAll("sim_data")

	// 3. Handle Ctrl+C to kill all nodes
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("\n[Launcher] Stopping all nodes...")
		for _, cmd := range cmds {
			if cmd.Process != nil {
				cmd.Process.Kill()
			}
		}
		os.Exit(0)
	}()

	// 4. Launch Genesis Node (Node 0)
	fmt.Println("[Launcher] Starting Genesis Node...")
	startNode(0, true, mainGoPath, frontendPath)

	// Wait for Genesis to start up
	time.Sleep(2 * time.Second)

	// 5. Launch Peers
	for i := 1; i < NodeCount; i++ {
		startNode(i, false, mainGoPath, frontendPath)
		time.Sleep(500 * time.Millisecond) // Stagger start
	}

	fmt.Printf("\n[Launcher] Network is running with %d nodes.\n", NodeCount)
	fmt.Printf("Genesis Dashboard: http://localhost:%d\n", StartHTTPPort)
	fmt.Println("Check 'sim_data/node_N/node.log' for output.")
	fmt.Println("Press Ctrl+C to stop.")

	select {} // Block forever
}

func startNode(id int, isGenesis bool, mainGoPath, frontendSrc string) {
	httpPort := StartHTTPPort + id
	udpPort := StartUDPPort + id

	// Define the node's isolated workspace
	nodeDir := filepath.Join("sim_data", fmt.Sprintf("node_%d", id))

	// A. Create Directory
	if err := os.MkdirAll(nodeDir, 0755); err != nil {
		panic(err)
	}

	// B. Copy Frontend Folder (So the UI works)
	destFrontend := filepath.Join(nodeDir, "frontend")
	if err := copyDir(frontendSrc, destFrontend); err != nil {
		fmt.Printf("Error copying frontend: %v\n", err)
	}

	// C. Construct the Command
	// go run main.go -port X -http Y [-genesis | -bootstrap Z]
	args := []string{
		"run",
		mainGoPath,
		"-port", strconv.Itoa(udpPort),
		"-http", strconv.Itoa(httpPort),
	}

	if isGenesis {
		args = append(args, "-genesis")
	} else {
		args = append(args, "-bootstrap", BootstrapAddr)
	}

	cmd := exec.Command("go", args...)
	cmd.Dir = nodeDir // Run INSIDE the node's folder (isolates private_key.pem)

	// D. Redirect Output to Log File
	logFile, _ := os.Create(filepath.Join(nodeDir, "node.log"))
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		panic(err)
	}

	cmds = append(cmds, cmd)
	fmt.Printf(" -> Node %d running (HTTP :%d / UDP :%d)\n", id, httpPort, udpPort)
}

// Recursive Copy Function
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(src, path)
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		return copyFile(path, dstPath)
	})
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}
