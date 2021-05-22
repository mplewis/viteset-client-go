// client provides a client for the Viteset API, allowing users to fetch blob values and subscribe to updates.
package client

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

// Client accesses a blob from Viteset and sends updates via a channel. To start using a Client, call
// `client.Subscribe()`.
type Client struct {
	Key      string        // The key for the blob
	Secret   string        // The secret for a client with access to the blob
	Interval time.Duration // The time between checking for updates
	Host     string        // The hostname of the Viteset API, default: Viteset production servers
	last     []byte        // The last-retrieved value for the blob
	ticker   *time.Ticker  // The ticker that checks for updates to the blob at an interval
}

// Update contains either a blob's new value, or an error that occurred during the last fetch. You must ensure
// `update.Error != nil` before reading the value. You may want to simply log and ignore errors; it is possible
// that a temporary network issue will resolve itself over time.
type Update struct {
	Value []byte
	Error error
}

// Subscribe starts watching the blob for changes. It returns a channel, via which updates to the blob value will
// be sent to the application, and an error if the subscription could not be created. The channel always emits the
// blob's initial value upon first fetch.
func (c *Client) Subscribe() (<-chan Update, error) {
	if c.ticker != nil {
		return nil, errors.New("cannot re-subscribe with a closed Client")
	}
	if c.Key == "" {
		return nil, errors.New("missing key")
	}
	if c.Secret == "" {
		return nil, errors.New("missing secret")
	}
	if c.Host == "" {
		c.Host = DEFAULT_HOST
	}
	if c.Interval == 0 {
		c.Interval = time.Duration(DEFAULT_INTERVAL)
	}

	ch := make(chan Update)
	ticker := time.NewTicker(c.Interval)
	go func() {
		for {
			data, err := c.fetch()
			if err != nil {
				ch <- Update{Error: err}
			} else if !bytes.Equal(data, c.last) {
				ch <- Update{Value: data}
				c.last = data
			}
			<-ticker.C
		}
	}()
	c.ticker = ticker
	return ch, nil
}

// Cancel cancels a subscription. No further updates will be sent on this channel. Do not reuse this Client or channel.
func (c *Client) Cancel() {
	if c.ticker != nil {
		c.ticker.Stop()
	}
}

// fetch retrieves the latest value for the blob.
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
