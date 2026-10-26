// Command sushi-latte is a thin root launcher for the real server.
//
// The API module lives in api/ (api/go.mod), so `go run ./api/cmd/server`
// from the repo root does not work. This launcher makes the repo
// runnable from the root with `go run .`: it starts the API server
// from api/ and passes through stdin/stdout/stderr and signals.
// The server itself serves the built frontends at /client-menu/,
// /staff/ and /display/ (build them first with `npm run build`,
// or use `npm run dev` for live frontend dev servers).
package main

import (
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

func main() {
	cmd := exec.Command("go", "run", "./cmd/server")
	cmd.Dir = "api"
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()
	if err := cmd.Start(); err != nil {
		log.Fatalf("starting API server: %v (run from the repo root, with Go 1.26+ on PATH)", err)
	}
	// Forward Ctrl+C / terminate to the server so it stops cleanly.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		_ = cmd.Process.Signal(syscall.SIGINT)
	}()
	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		log.Fatal(err)
	}
}
