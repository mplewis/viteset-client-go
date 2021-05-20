package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

const DEFAULT_HOST = "https://api.viteset.com"
const DEFAULT_INTERVAL = time.Minute

type Client struct {
	Host     string
	Key      string
	Secret   string
	Interval time.Duration
	OnUpdate func(data []byte, err error)
	last     []byte
}

func (c *Client) Subscribe() error {
	if c.Key == "" {
		return errors.New("missing key")
	}
	if c.Secret == "" {
		return errors.New("missing secret")
	}
	if c.Host == "" {
		c.Host = DEFAULT_HOST
	}
	if c.Interval == 0 {
		c.Interval = time.Duration(DEFAULT_INTERVAL)
	}

	ticker := time.NewTicker(c.Interval)
	go func() {
		for {
			data, err := c.fetch()
			if err != nil {
				c.OnUpdate(data, err)
			} else if !bytes.Equal(data, c.last) {
				c.last = data
				c.OnUpdate(data, err)
			}
			<-ticker.C
		}
	}()

	return nil
}

func (c *Client) fetch() ([]byte, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/%s", c.Host, c.Key), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.Secret))
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return ioutil.ReadAll(resp.Body)
}
