// Command zebra-config is a local setup tool for Zebra ZD410 and ZD411 printers.
// Double-click the program: it listens on 127.0.0.1 and opens a browser.
// It does not scan the network and it does not open any port except the
// printer IP typed into the page.
package main

import (
	"flag"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/manovi-code/zebra-config/internal/httpserver"
	"github.com/manovi-code/zebra-config/internal/printer"
)

func main() {
	port := flag.Int("port", 17891, "local page port; if it is busy, a free port is used")
	noBrowser := flag.Bool("no-browser", false, "do not open a browser")
	configDir := flag.String("config", "", "folder for the recent-printers file (default: the user config folder)")
	flag.Parse()

	recentPath := ""
	if *configDir != "" {
		recentPath = filepath.Join(*configDir, "recent.json")
	}
	srv, err := httpserver.New(recentPath, printer.DefaultTimeouts())
	if err != nil {
		log.Fatal(err)
	}
	addr, err := listen(srv, *port)
	if err != nil {
		log.Fatal(err)
	}
	url := "http://" + addr + "/"
	fmt.Println("Zebra Config is running at", url)
	fmt.Println("Close this window to quit.")
	fmt.Println("This program only listens on this computer. It does not scan the network.")
	if !*noBrowser {
		if err := openBrowser(url); err != nil {
			fmt.Println("Open that address in a browser. The browser did not open on its own.")
		}
	}
	if err := srv.Serve(); err != nil {
		log.Fatal(err)
	}
}

func listen(srv *httpserver.Server, port int) (string, error) {
	if port == 0 {
		return srv.Listen("127.0.0.1:0")
	}
	addr, err := srv.Listen(fmt.Sprintf("127.0.0.1:%d", port))
	if err == nil {
		return addr, nil
	}
	fmt.Printf("Port %d is in use. Using a free port instead.\n", port)
	return srv.Listen("127.0.0.1:0")
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
