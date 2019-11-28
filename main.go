package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"

	"github.com/go-redis/redis"
)

var (
	Version, Build string
	conf           *Config
)

type RedisOption struct {
	Addr     string `json:"addr"`
	Password string `json:"password"`
}

type Config struct {
	Host   string        `json:"host"`
	AesKey string        `json:"aeskey"`
	Token  string        `json:"token"`
	Redis  redis.Options `json:"redis"`
	Errmsg []string      `json:"errmsg"`
}

func loadConfig(fname string) {
	c := &Config{}
	f, err := os.Open(fname)
	if err != nil {
		log.Fatalf("open config file fail %s", err.Error())
	}
	if err = json.NewDecoder(f).Decode(c); err != nil {
		log.Fatalf("parse config file fail %s", err.Error())
	}
	conf = c
}

func main() {
	version := flag.Bool("version", false, "build version")
	confile := flag.String("config", "config.json", "config file")
	flag.Parse()
	if *version {
		fmt.Printf("Version: %s Build: %s\nGo Version: %s\nGo OS/ARCH: %s %s\n", Version, Build, runtime.Version(), runtime.GOOS, runtime.GOARCH)
		return
	}
	loadConfig(*confile)

	s, err := newServer()
	if err != nil {
		log.Fatalf("Error creating server: %v", err)
	}
	log.Printf("Listening on :%v ...", conf.Host)
	log.Fatalf("Error listening on %v: %v", conf.Host, http.ListenAndServe(conf.Host, s))
}
