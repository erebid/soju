package main

import (
	"crypto/tls"
	"flag"
	"log"
	"net"
	"net/http"

	"git.sr.ht/~emersion/soju"
	"git.sr.ht/~emersion/soju/config"
)

func main() {
	var addr, configPath string
	var debug bool
	flag.StringVar(&addr, "listen", "", "listening address")
	flag.StringVar(&configPath, "config", "", "path to configuration file")
	flag.BoolVar(&debug, "debug", false, "enable debug logging")
	flag.Parse()

	var cfg *config.Server
	if configPath != "" {
		var err error
		cfg, err = config.Load(configPath)
		if err != nil {
			log.Fatalf("failed to load config file: %v", err)
		}
	} else {
		cfg = config.Defaults()
	}

	if addr != "" {
		cfg.Addr = addr
	}

	db, err := soju.OpenSQLDB(cfg.SQLDriver, cfg.SQLSource)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}

	var ln net.Listener
	if cfg.TLS != nil {
		cert, err := tls.LoadX509KeyPair(cfg.TLS.CertPath, cfg.TLS.KeyPath)
		if err != nil {
			log.Fatalf("failed to load TLS certificate and key: %v", err)
		}

		tlsCfg := &tls.Config{Certificates: []tls.Certificate{cert}}
		ln, err = tls.Listen("tcp", cfg.Addr, tlsCfg)
		if err != nil {
			log.Fatalf("failed to start TLS listener: %v", err)
		}
	} else {
		var err error
		ln, err = net.Listen("tcp", cfg.Addr)
		if err != nil {
			log.Fatalf("failed to start listener: %v", err)
		}
	}

	srv := soju.NewServer(db)
	// TODO: load from config/DB
	srv.Hostname = cfg.Hostname
	srv.LogPath = cfg.LogPath
	srv.Debug = debug

	log.Printf("server listening on %q", cfg.Addr)
	go func() {
		if err := srv.Run(); err != nil {
			log.Fatal(err)
		}
	}()
	go func() {
		httpSrv := http.Server{
			Addr: ":8080",
			Handler: srv,
		}
		httpSrv.ListenAndServe()
	}()
	log.Fatal(srv.Serve(ln))
}
