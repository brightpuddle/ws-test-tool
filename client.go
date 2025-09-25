package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tidwall/gjson"
)

// client is an ACI API client
type client struct {
	host                    string
	usr                     string
	pwd                     string
	url                     url.URL
	httpClient              *http.Client
	lastRefresh             time.Time
	lastSubscriptionRefresh time.Time
	subscriptionID          string
}

// newACIClient configures and returns a new ACI API client
func newACIClient(a args) *client {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	cookieJar, _ := cookiejar.New(nil)
	httpClient := &http.Client{
		Timeout:   time.Duration(a.HTTPTimeout) * time.Second,
		Transport: tr,
		Jar:       cookieJar,
	}
	return &client{
		httpClient: httpClient,
		host:       a.APIC,
		usr:        a.Usr,
		pwd:        a.Pwd,
		url:        url.URL{Scheme: "https", Host: a.APIC},
	}
}

// token gets the login token from the current cookie
func (c *client) token() string {
	for _, cookie := range c.httpClient.Jar.Cookies(&c.url) {
		if cookie.Name == "APIC-cookie" {
			return cookie.Value
		}
	}
	return ""
}

// login authenticates the fabric
func (c *client) login() error {
	log.Info().Msgf("Logging in to %s", c.host)
	data := json{}.set("aaaUser.attributes", map[string]string{
		"name": c.usr,
		"pwd":  c.pwd,
	}).str
	res, err := c.httpClient.Post(c.url.String()+"/api/aaaLogin.json",
		"application/json",
		strings.NewReader(data))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		panic(fmt.Sprintf("HTTP status code: %d", res.StatusCode))
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	record := gjson.GetBytes(body, "imdata.0")
	errText := record.Get("error.attributes.text").Str
	if errText != "" {
		return errors.New(errText)
	}
	c.lastRefresh = time.Now()
	return nil
}

// refresh refreshes the login token
func (c *client) refresh() error {
	log.Debug().Msg("Refreshing login token")
	res, err := c.httpClient.Get(c.url.String() + "/api/aaaRefresh.json")
	if err != nil {
		return err
	}
	defer res.Body.Close()
	c.lastRefresh = time.Now()
	return nil
}

func (c *client) refreshLoop() error {
	for {
		elapsed := time.Since(c.lastRefresh)
		limit := 60 * time.Second
		if elapsed > limit {
			if err := c.refresh(); err != nil {
				return err
			}
		}
		time.Sleep(1 * time.Second)
	}
}

// connectSocket connects the websocket
func (c *client) connectSocket() (*websocket.Conn, error) {
	log.Info().Msg("Connecting websocket")

	wsURL := url.URL{
		Scheme: "wss",
		Host:   c.host,
		Path:   "/socket" + c.token(),
	}

	dialer := *websocket.DefaultDialer
	dialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	ws, _, err := dialer.Dial(wsURL.String(), nil)
	return ws, err
}

// listenSocket listents for incoming websocket messages
func (c *client) listenSocket(ws *websocket.Conn) error {
	log.Info().Msg("Listening for incoming messages")
	defer ws.Close()
	for {
		_, msg, err := ws.ReadMessage()
		if err != nil {
			return err
		}
		if gjson.ValidBytes(msg) {
			fmt.Println(gjson.ParseBytes(msg).Get("@pretty"))
		} else {
			log.Warn().Msgf("Non-JSON msg rcvd: %s\n", msg)
		}
	}
}

// subscribe subscribes to REP faults
func (c *client) subscribe(class string, params map[string]string) error {
	log.Info().Msgf("Susbscribing to %s", class)
	queryValues := url.Values{}
	queryValues.Add("subscription", "yes")
	for k, v := range params {
		queryValues.Add(k, v)
	}

	u := url.URL{
		Scheme:   c.url.Scheme,
		Host:     c.url.Host,
		Path:     fmt.Sprintf("/api/class/%s.json", class),
		RawQuery: queryValues.Encode(),
	}

	res, err := c.httpClient.Get(u.String())
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	jsonRes := gjson.ParseBytes(body)
	if errStr := jsonRes.Get("imdata.0.error.attributes.text").Str; errStr != "" {
		return errors.New(errStr)
	}
	subscriptionID := jsonRes.Get("subscriptionId").Str
	if subscriptionID == "" {
		return errors.New("no subscription ID in reply")
	}
	c.subscriptionID = subscriptionID
	c.lastSubscriptionRefresh = time.Now()
	return nil
}

func (c *client) refreshSubscription() error {
	log.Debug().Msg("Refreshing subscription")
	u := fmt.Sprintf("%s/api/subscriptionRefresh.json?id=%s",
		c.url.String(),
		c.subscriptionID,
	)
	res, err := c.httpClient.Get(u)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	errStr := gjson.GetBytes(body, "imdata.0.error.attributes.text").Str
	if errStr != "" {
		return errors.New(errStr)
	}
	c.lastSubscriptionRefresh = time.Now()
	return nil
}

func (c *client) subscriptionRefreshLoop() error {
	log.Info().Msg("Starting subscription refresh loop")
	for {
		elapsed := time.Since(c.lastSubscriptionRefresh)
		limit := 30 * time.Second
		if elapsed > limit {
			if err := c.refreshSubscription(); err != nil {
				return err
			}
		}
		time.Sleep(1 * time.Second)
	}
}
