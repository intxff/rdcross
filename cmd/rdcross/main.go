package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os/signal"
	"syscall"
	"time"

	"github.com/intxff/rdcross/config"
	"github.com/intxff/rdcross/global"
	"github.com/intxff/rdcross/log"
	"go.uber.org/zap"
)

const (
	_version = 0.1
)

var (
	path    string
	version bool
	test    bool
)

func init() {
	flag.StringVar(&path, "c", "./config.yaml", "path of yaml configuration file")
	flag.BoolVar(&version, "v", false, "version")
	flag.BoolVar(&test, "t", false, "test config file")
	flag.Parse()
}

func main() {
	go func() {
		http.ListenAndServe(":6060", nil)
	}()

	if version {
		fmt.Printf("rdcross: %v\n", _version)
	}

	// parse config
	rdConfig, err := config.ParseRawConfig(path)
	if err != nil {
		log.Fatal("[Config] failed to unmarshal config", zap.Error(err))
	}
	if err := global.Init(rdConfig); err != nil {
		log.Fatal("[Global] failed to init global resouce", zap.Error(err))
	}
	if test {
		return
	}

	Run()
}

func Run() {
	ctx, _ := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	// init
	g := global.New()

	// dns
	go g.DNS.ListenAndServe()

	// ingress
	for _, v := range g.Ingress {
		go v.Run(*g.Router)
	}

	// restful api

	<-ctx.Done()
	log.Info("[EXIT] Closing")
	//close all
	closeall := func() <-chan struct{} {
		g.DNS.Shutdown()
		ch := make(chan struct{}, 1)
		for _, v := range g.Ingress {
			<-v.Close()
		}
		for _, v := range g.Egress {
			<-v.Close()
		}
		ch <- struct{}{}
		return ch
	}

	select {
	case <-time.After(time.Second * 5):
		log.Error("[EXIT] timeout, force closing\n")
	case <-closeall():
	}
	log.Info("[EXIT] Bye")
}
