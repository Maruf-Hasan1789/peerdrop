package main

import (
	"context"
	"embed"
	"flag"
	"log"
	"os"
	"time"

	"github.com/Maruf-Hasan1789/peerdrop/internal/app"
	"github.com/Maruf-Hasan1789/peerdrop/internal/discovery"
	"github.com/Maruf-Hasan1789/peerdrop/internal/protocol"
	"github.com/Maruf-Hasan1789/peerdrop/internal/transfer"
	transport "github.com/Maruf-Hasan1789/peerdrop/internal/transport/tcp"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

var wailsApp *app.App

//go:embed all:frontend/dist
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

	settings, err := app.LoadOrCreateSettings()

	if err != nil {
		log.Printf("Error loading settings: %v", err)
	}
	var userName string

	if settings.UserName == "" {
		userName = selfPeer.Name
	} else {
		userName = settings.UserName
	}

	selfPeer.UserName = userName
	//start registering
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		for range ticker.C {
			if err := d.Register(ctx, selfPeer, userName); err != nil {
				log.Println(err)
				cancel()
			}
		}
	}()

	//start browsing
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		for range ticker.C {
			err := d.Browse(ctx, *selfPeer)
			if err != nil {
				log.Println(err)
				cancel()
			}
		}
	}()

	if err != nil {
		log.Printf("Error while listening")
	}
	transferRegistry := transfer.NewRegistry()

	wailsApp = app.NewApp(d, transferRegistry)
	d.AddObserver(wailsApp)

	permissionManager := app.NewPermissionManager(settings)
	handshakeOptions := &protocol.HandshakeOptions{
		PermissionFunc: permissionManager.Request,
	}

	err = wails.Run(&options.App{
		Title:  "PeerDrop",
		Width:  1920,
		Height: 1080,
		OnStartup: func(ctx context.Context) {
			wailsApp.Startup(ctx, settings, permissionManager)

			go func() {
				for {
					conn, err := listener.Accept(ctx, selfPeer, handshakeOptions)

					if err != nil {
						log.Printf("Error while getting connection from listener %v\n", err)
						continue
					}

					go wailsApp.HandleConnection(conn)
				}
			}()
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
