package main

import (
	"time"

	"github.com/rs/zerolog"
)

const (
	sleepDuration = 30 * time.Second
)

var (
	version string
	log     zerolog.Logger
)

func sleep(t time.Duration) {
	log.Info().Msgf("Pausing for %s", t)
	time.Sleep(t)
}

func main() {
	args := newArgs()
	log = newLogger(args.Class + ".log")
	client := newACIClient(args)

	errChan := make(chan error)

	for {

		// Login
		if err := client.login(); err != nil {
			log.Error().Err(err).Msg("Login error")
			errChan <- err
			continue
		}

		// Start token refresh
		if client.token() != "" {
			go func() {
				err := client.refreshLoop()
				if err != nil {
					log.Error().Err(err).Msg("Refresh error")
					errChan <- err
				}
			}()
		}

		// Connect to socket
		ws, err := client.connectSocket()
		if err != nil {
			log.Error().Err(err).Msg("Websocket connection error")
			errChan <- err
			continue
		}

		// Start websocket listener
		go func() {
			if err := client.listenSocket(ws); err != nil {
				log.Error().Err(err).Msg("Websocket error")
				errChan <- err
			}
		}()

		// Subscribe
		if err := client.subscribe(args.Class, map[string]string{
			"page":      "0",
			"page-size": "1",
		}); err != nil {
			log.Error().Err(err).Msg("Subscription error")
			errChan <- err
			continue
		}

		// Start subscription refresh
		if client.subscriptionID != "" {
			go func() {
				if err := client.subscriptionRefreshLoop(); err != nil {
					log.Error().Err(err).Msg("Subscription refresh error")
					errChan <- err
				}
			}()
		}

		// Listen for errors, rince, and repeat
		if err := <-errChan; err != nil {
			log.Debug().Err(err).Msg("Restarting due to error")
			sleep(sleepDuration)
		}
	}
}
