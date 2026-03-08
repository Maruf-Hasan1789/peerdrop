package main

import (
	"context"
	"embed"
	"log"
	"time"

	"github.com/Maruf-Hasan1789/peerdrop/internal/app"
	"github.com/Maruf-Hasan1789/peerdrop/internal/discovery"
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	listener, err := transport.NewListener()

	if err != nil {
		log.Fatal("Error while creating listener ", err)
	}

	port := listener.GetPort()

	if port == -1 {
		log.Fatal("Error while creating listener port ", port)
	}

	selfPeer := discovery.GetSelfPeer(port)

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
		err := d.Register(ctx, selfPeer, userName)
		if err != nil {
			log.Printf("Error registering peer: %v", err)
			cancel()
		}
		/*ticker := time.NewTicker(30 * time.Second)
		for range ticker.C {
			if err := d.Register(ctx, selfPeer, userName); err != nil {
				log.Println(err)
				cancel()
			}
		}

		*/
	}()

	//start browsing
	go func() {

		err := d.Browse(ctx, *selfPeer)
		if err != nil {
			log.Println(err)
			cancel()
		}

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

	wailsApp = app.NewApp(d, transferRegistry, selfPeer)
	d.AddObserver(wailsApp)

	//permissionManager := app.NewPermissionManager(settings)
	//handshakeOptions := &protocol.HandshakeOptions{
	//	PermissionFunc: permissionManager.Request,
	//}

	err = wails.Run(&options.App{
		Title:  "PeerDrop",
		Width:  1920,
		Height: 1080,
		OnStartup: func(ctx context.Context) {
			wailsApp.Startup(ctx, settings)

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

	log.Printf("Listening on port %v\n", port)

}
