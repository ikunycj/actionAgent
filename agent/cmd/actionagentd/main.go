package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"actionagent/agent/kernel"
)

func main() {
	var cfgPath string
	var addr string
	flag.StringVar(&cfgPath, "config", "", "config file path")
	flag.StringVar(&addr, "addr", "", "http bind address override")
	flag.Parse()

	exe, _ := os.Executable()
	rt := kernel.NewRuntime(kernel.StartOptions{
		CLIConfigPath: cfgPath,
		BinaryPath:    exe,
		AppName:       "ActionAgent",
		HTTPAddr:      addr,
		EnvVarName:    "ACTIONAGENT_CONFIG",
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := rt.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "startup failed: %v\n", err)
		os.Exit(1)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	shutdownCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	_ = rt.Shutdown(shutdownCtx)
}
