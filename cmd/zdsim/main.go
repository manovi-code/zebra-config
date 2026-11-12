// Command zdsim pretends to be a Zebra printer on a TCP port so Zebra Config
// can be tried without hardware. Settings live in memory only. It has no
// label formats, graphics, or file list.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/manovi-code/zebra-config/internal/sim"
)

func main() {
	port := flag.Int("port", 9100, "TCP port (the printer default is 9100; use 9101 if 9100 is taken)")
	model := flag.String("model", "ZD411", "model to report, for example ZD411, ZD410, or ZD621")
	dpi := flag.Int("dpi", 203, "resolution to report, for example 203 or 300")
	delay := flag.Duration("delay", 0, "wait this long before each reply, for example 5s")
	hang := flag.Bool("hang", false, "accept the connection and never answer (timeout)")
	refuse := flag.Bool("refuse", false, "do not listen, so the connection is refused")
	flag.Parse()

	if *refuse {
		fmt.Printf("Refusing connections on 127.0.0.1:%d. Nothing is listening, so the computer should report connection refused.\n", *port)
		fmt.Println("Press Ctrl+C to stop.")
		select {}
	}
	srv, err := sim.Listen(fmt.Sprintf("127.0.0.1:%d", *port), sim.Options{
		Model: *model,
		DPI:   *dpi,
		Delay: *delay,
		Hang:  *hang,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "Could not listen:", err)
		os.Exit(1)
	}
	defer srv.Close()
	fmt.Printf("Pretend %s at %d dpi on %s\n", *model, *dpi, srv.Addr())
	if *hang {
		fmt.Println("Hang mode: connections are accepted and no reply is sent.")
	}
	if *delay > 0 {
		fmt.Println("Each reply waits", *delay)
	}
	fmt.Println("Press Ctrl+C to stop. Settings are forgotten when this stops.")
	select {}
}
