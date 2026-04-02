package main

import (
	"fmt"
	"os"
)

const version = "0.1.0"

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	switch args[0] {
	case "--help", "-h":
		printUsage()
		return
	case "--version", "-v":
		fmt.Println("whoisusing", version)
		return
	default:
		fmt.Println("TODO: port lookup not implemented yet")
	}
}

func printUsage() {
	fmt.Println(`whoisusing — find what's using a port

Usage:
  whoisusing <port>       Show process using a specific port
  whoisusing --help       Show this help
  whoisusing --version    Show version

Examples:
  whoisusing 8080
  whoisusing 3000`)
}
