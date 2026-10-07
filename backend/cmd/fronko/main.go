// Command fronko is the Fronko API server.
//
//	fronko                 run the server (the default)
//	fronko admin …         manage platform admins
//	fronko seed [--reset]  fill a development database with demo data
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/faiz-gh/fronko/backend/internal/devseed"
	"github.com/faiz-gh/fronko/backend/internal/platform/config"
	"github.com/faiz-gh/fronko/backend/internal/platformadmin"
	"github.com/faiz-gh/fronko/backend/internal/server"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "admin":
			exitOn(platformadmin.RunCLI(os.Args[2:]))
			return
		case "seed":
			exitOn(devseed.RunCLI(os.Args[2:]))
			return
		case "serve":
		default:
			exitOn(fmt.Errorf("unknown command %q (try: serve, admin, seed)", os.Args[1]))
		}
	}

	log.Println("Starting Fronko backend...")
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := server.Run(ctx, cfg); err != nil {
		log.Fatal(err)
	}
}

func exitOn(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
