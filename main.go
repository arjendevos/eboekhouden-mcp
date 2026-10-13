// Command eboekhouden-mcp is an MCP server for the e-Boekhouden.nl API. It
// speaks MCP over stdio.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/pointers-agency/eboekhouden-mcp/internal/eboekhouden"
	"github.com/pointers-agency/eboekhouden-mcp/internal/server"
)

var version = "dev"

func main() {
	log.SetFlags(0)
	log.SetPrefix("eboekhouden-mcp: ")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}

	token := os.Getenv("EBOEKHOUDEN_API_TOKEN")
	if token == "" {
		log.Fatal("EBOEKHOUDEN_API_TOKEN is not set; create an API token in the settings of your e-Boekhouden account")
	}
	source := os.Getenv("EBOEKHOUDEN_SOURCE")
	if source == "" {
		source = "MCP"
	}
	readOnly, _ := strconv.ParseBool(os.Getenv("EBOEKHOUDEN_READ_ONLY"))

	client := &eboekhouden.Client{
		BaseURL:  os.Getenv("EBOEKHOUDEN_BASE_URL"),
		APIToken: token,
		Source:   source,
	}
	s, err := server.New(client, server.Options{Version: version, ReadOnly: readOnly})
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	runErr := s.Run(ctx, &mcp.StdioTransport{})

	closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = client.Close(closeCtx)
	if runErr != nil && ctx.Err() == nil {
		log.Fatal(runErr)
	}
}
