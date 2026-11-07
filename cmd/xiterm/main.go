// XiTerm is a native terminal built with Onix. Linux, macOS and Windows are supported.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/diiyw/xiterm/internal/app"
	"github.com/diiyw/xiterm/internal/config"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
func run() error {
	defaultPath, err := config.Path()
	if err != nil {
		return err
	}
	path := flag.String("config", defaultPath, "path to config.json")
	directory := flag.String("directory", "", "working directory for the initial shell")
	printConfig := flag.Bool("print-default-config", false, "print default configuration and exit")
	smoke := flag.Duration("smoke-duration", 0, "close automatically after this duration (for native smoke tests)")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println("XiTerm " + version)
		return nil
	}
	if *printConfig {
		b, err := json.MarshalIndent(config.Default(), "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	}
	if *smoke < 0 || *smoke > time.Minute {
		return fmt.Errorf("smoke-duration must be between 0 and 1m")
	}
	cfg, err := config.Load(*path)
	if err != nil {
		return err
	}
	if *directory != "" {
		*directory, err = filepath.Abs(*directory)
		if err != nil {
			return err
		}
		info, err := os.Stat(*directory)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("%s is not a directory", *directory)
		}
	}
	return app.Run(app.Options{Config: cfg, ConfigPath: *path, Directory: *directory, SmokeDuration: *smoke})
}
