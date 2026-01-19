package main

import (
	"context"
	"embed"
	"flag"
	"log"
	"os"

	"github.com/Maruf-Hasan1789/peerdrop/internal/app"
	"github.com/Maruf-Hasan1789/peerdrop/internal/discovery"
	transport "github.com/Maruf-Hasan1789/peerdrop/internal/transport/tcp"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

var wailsApp *app.App
var assets embed.FS

func main() {
	port := flag.Int("port", 6623, "port")
	flag.Parse()
	log.Printf("Os Args %s", os.Args)
	log.Printf("Port %v\n", *port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	selfPeer := discovery.GetSelfPeer(*port)

	listener, err := transport.NewListener(selfPeer)

	d := discovery.New()

	//start registering
	go func() {
		if err := d.Register(ctx, selfPeer); err != nil {
			log.Println(err)
			cancel()
		}
	}()

	//start browsing
	go func() {
		err := d.Browse(ctx, *selfPeer)
		if err != nil {
			log.Println(err)
			cancel()
		}
	}()

	if err != nil {
		log.Printf("Error while listening")
	}

	wailsApp = app.NewApp(d)
	d.AddObserver(wailsApp)

	go func() {
		for {
			conn, err := listener.Accept(ctx, selfPeer)

			if err != nil {
				log.Printf("Error while getting connection from listener %v\n", err)
				continue
			}

			go wailsApp.HandleConnection(conn)
		}
	}()

	err = wails.Run(&options.App{
		Title:  "PeerDrop",
		Width:  1024,
		Height: 768,
		OnStartup: func(ctx context.Context) {
			wailsApp.Startup(ctx)
		},
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop:     true,
			DisableWebViewDrop: false,
		},
		Bind: []interface{}{
			wailsApp,
		},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
	})
	if err != nil {
		log.Printf("Error :%v\n", err)
		cancel()
	}

	log.Printf("Listening on port %v\n", *port)

}
