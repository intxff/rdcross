// modified from clash source code
package mmdb

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"

	"github.com/intxff/rdcross/log"
	"github.com/oschwald/geoip2-golang"
	"go.uber.org/zap"
)

var (
	mmdb *geoip2.Reader
	once sync.Once
)

func downloadMMDB(path string) (err error) {
	resp, err := http.Get("https://cdn.jsdelivr.net/gh/Dreamacro/maxmind-geoip@release/Country.mmdb")
	if err != nil {
		return
	}
	defer resp.Body.Close()

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)

	return err
}

func InitMMDB(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		log.Info("Can't find MMDB, start download")
		if err := downloadMMDB(path); err != nil {
			return fmt.Errorf("can't download MMDB: %s", err.Error())
		}
	}

	if !verify(path) {
		log.Error("MMDB invalid, remove and download")
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("can't remove invalid MMDB: %s", err.Error())
		}

		if err := downloadMMDB(path); err != nil {
			return fmt.Errorf("can't download MMDB: %s", err.Error())
		}
	}

	return nil
}

func verify(path string) bool {
	instance, err := geoip2.Open(path)
	if err == nil {
		instance.Close()
	}
    defer instance.Close()

	return err == nil
}

func Instance(path string) *geoip2.Reader {
	once.Do(func() {
		var err error
		mmdb, err = geoip2.Open(path)
		if err != nil {
			log.Panic("Can't load mmdb", zap.Error(err))
		}
	})

	return mmdb
}
