// client provides a client for the Viteset API, allowing users to fetch blob values and subscribe to updates.
package client

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

const VERSION = "1.0.0"
const DEFAULT_HOST = "https://api.viteset.com"
const DEFAULT_INTERVAL = time.Minute

var userAgent = fmt.Sprintf("Viteset-Client-Go/%s", VERSION)

// Client accesses a blob from Viteset and sends updates via a channel. To start using a Client, call
// `client.Subscribe()`.
type Client struct {
	Secret   string        // The secret for a client with access to the specified blob
	Blob     string        // The name of the blob to subscribe to
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
	if c.Blob == "" {
		return nil, errors.New("missing blob name")
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

	var lastEtag *string = nil
	ch := make(chan Update)
	c.ticker = time.NewTicker(c.Interval)

	go func() {
		for {
			same, data, etag, err := c.fetch(lastEtag)
			if err != nil {
				// something went wrong
				ch <- Update{Error: err}
			} else if same {
				// value has not changed; do nothing
			} else {
				// value has changed
				ch <- Update{Value: data}
				c.last = data
				lastEtag = etag
			}
			<-c.ticker.C
		}
	}()

	return ch, nil
}

// Cancel cancels a subscription. No further updates will be sent on this channel. Do not reuse this Client or channel.
func (c *Client) Cancel() {
	if c.Active() {
		c.ticker.Stop()
		c.ticker = nil
	}
}

// Active returns True if this Client is actively subscribed to a blob and False if the subscription has been Canceled.
func (c *Client) Active() bool {
	return c.ticker != nil
}

// fetch retrieves the latest value for the blob, obeying caching logic if we have a copy of this blob from the past.
func (c *Client) fetch(lastEtag *string) (same bool, data []byte, etag *string, err error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/%s", c.Host, c.Blob), nil)
	if err != nil {
		return false, nil, nil, err
	}
	req.Header.Add("User-Agent", userAgent)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.Secret))
	if lastEtag != nil {
		req.Header.Add("If-None-Match", *lastEtag)
	}
	resp, err := client.Do(req)
	if err != nil {
		return false, nil, nil, err
	}
	data, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return false, nil, nil, err
	}
	if resp.StatusCode == http.StatusNotModified {
		return true, nil, nil, err
	}
	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("expected status code %d but got %d: `%s`", http.StatusOK, resp.StatusCode, data)
		return false, nil, nil, err
	}
	t := resp.Header.Get("ETag")
	return false, data, &t, err
}
