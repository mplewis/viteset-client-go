// A client for the Viteset API, allowing users to fetch blob values and subscribe to updates.
//
// To get started, create a Client for your target blob, then call Subscribe to start receiving updates:
//
//     c := Client{
//         Blob:   "SOME_BLOB_NAME",
//         Secret: "SOME_CLIENT_SECRET",
//     }
//
//     updates, err := c.Subscribe()
//     if err != nil {
//         // Uh-oh! Subscription failed. Check your Blob and Secret values.
//         log.Panic(err)
//     }
//
//     for {
//         update := <-updates
//         if update.err != nil {
//             // Failure to fetch an update isn't all that bad.
//             // Just use the last cached value for now.
//             log.Println(err)
//             break
//         }
//
//         // Blob values are provided as []byte,
//         // which you can pass to your parser of choice
//         updateMyAppConfig(update.Value)
//     }
package client

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

// The package version.
const VERSION = "1.0.0"

// The default Viteset host to fetch blobs from.
const DEFAULT_HOST = "https://api.viteset.com"

// The default interval for polling for blob updates.
const DEFAULT_INTERVAL = 15 * time.Second

var userAgent = fmt.Sprintf("Viteset-Client-Go/%s", VERSION)

// Client accesses a blob from Viteset and sends updates via a channel.
// The Client uses ETags to minimize data received when the blob hasn't changed since the last poll.
type Client struct {
	// The secret for a client with access to the specified blob
	Secret string

	// The name of the blob to subscribe to
	Blob string

	// Optional: The update polling interval. Default is 15 seconds.
	// Please don't reduce this below 15 seconds: this greatly impacts load on Viteset servers.
	Interval time.Duration

	// Optional: The hostname of the Viteset API. Default is Viteset production servers.
	Host string

	// The last-retrieved value for the blob
	last []byte

	// The ticker that polls for updates to the blob at an interval
	ticker *time.Ticker
}

// Update contains either a blob's latest value, or an error that occurred during the last fetch. You must check
// update.Error before reading the value.
//
// You may want to simply log and ignore these errors.
// Temporary network issues will likely resolve themselves over time.
type Update struct {
	Value []byte
	Error error
}

// Subscribe starts watching the blob for changes. It returns a channel and an error.
//
// If the subscription is successful, the Client returns an Update channel,
// sends the initial blob value into the channel, and sends any future blob values into the channel.
//
// If the subscription is unsuccessful, the error will be set.
//
// On a successful subscription, the Client will always send the initial value of the blob via the channel.
func (c *Client) Subscribe() (<-chan Update, error) {
	if c.Active() {
		return nil, errors.New("client subscription is already active")
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

// Cancel cancels a subscription. This Client will stop polling, and no further updates will be sent on its channel.
//
// Reusing a canceled Client is not supported.
func (c *Client) Cancel() {
	if c.Active() {
		c.ticker.Stop()
		c.ticker = nil
	}
}

// Active returns True if this Client is actively subscribed to a blob and False otherwise.
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
