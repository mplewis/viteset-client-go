package main_test

import (
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	viteset "github.com/mplewis/viteset-client-go"
)

// Live test. Use `go test` and set the env vars below to try the library out.
func TestLive(t *testing.T) {
	secret := os.Getenv("SECRET")
	blob := os.Getenv("BLOB")
	host := os.Getenv("HOST")
	if secret == "" {
		log.Fatal("Must provide SECRET env var")
	}
	if blob == "" {
		log.Fatal("Must provide BLOB env var")
	}
	if host == "" {
		host = "https://api.viteset.com"
	}
	c := viteset.Client{
		Secret:   secret,
		Blob:     blob,
		Interval: 15 * time.Second,
		Host:     host,
	}
	ch, err := c.Subscribe()
	if err != nil {
		log.Panic(err)
	}
	for {
		data := <-ch
		fmt.Printf("Update: value: %s, error: %+v\n", data.Value, data.Error)
	}
}
