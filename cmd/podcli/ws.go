package main

import (
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/chzyer/readline"
	"github.com/fatih/color"
	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var origin string

func init() {
	wsCmd.Flags().StringVarP(&origin, "origin", "o", "", "websocket origin")
	rootCmd.AddCommand(wsCmd)
}

var wsCmd = &cobra.Command{
	Use:     `ws [address]`,
	Short:   "Websocket client",
	Example: `  ws localhost:9898/ws/echo`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return fmt.Errorf("address is required")
		}

		address := args[0]
		if !strings.HasPrefix(address, "ws://") && !strings.HasPrefix(address, "wss://") {
			address = fmt.Sprintf("ws://%s", address)
		}

		dest, err := url.Parse(address)
		if err != nil {
			return err
		}
		if origin != "" {
		} else {
			originURL := *dest
			if dest.Scheme == "wss" {
				originURL.Scheme = "https"
			} else {
				originURL.Scheme = "http"
			}
			origin = originURL.String()
		}

		err = connect(dest.String(), origin, &readline.Config{
			Prompt: "> ",
		})
		if err != nil {
			logger.Info("websocket closed", zap.Error(err))
		}
		return nil
	},
}

type session struct {
	ws      *websocket.Conn
	rl      *readline.Instance
	errChan chan error
}

func connect(url, origin string, rlConf *readline.Config) error {
	headers := make(http.Header)
	headers.Add("Origin", origin)

	ws, _, err := websocket.DefaultDialer.Dial(url, headers)
	if err != nil {
		return err
	}

	rl, err := readline.NewEx(rlConf)
	if err != nil {
		return err
	}
	defer rl.Close()

	sess := &session{
		ws:      ws,
		rl:      rl,
		errChan: make(chan error),
	}

	go sess.readConsole()
	go sess.readWebsocket()

	return <-sess.errChan
}

func (s *session) readConsole() {
	for {
		line, err := s.rl.Readline()
		if err != nil {
			s.errChan <- err
			return
		}

		err = s.ws.WriteMessage(websocket.TextMessage, []byte(line))
		if err != nil {
			s.errChan <- err
			return
		}
	}
}

func bytesToFormattedHex(bytes []byte) string {
	text := hex.EncodeToString(bytes)
	return regexp.MustCompile("(..)").ReplaceAllString(text, "$1 ")
}

func (s *session) readWebsocket() {
	rxSprintf := color.New(color.FgGreen).SprintfFunc()

	for {
		msgType, buf, err := s.ws.ReadMessage()
		if err != nil {
			fmt.Fprint(s.rl.Stdout(), rxSprintf("< %s\n", err.Error()))
			os.Exit(1)
			return
		}

		var text string
		switch msgType {
		case websocket.TextMessage:
			text = string(buf)
		case websocket.BinaryMessage:
			text = bytesToFormattedHex(buf)
		default:
			s.errChan <- fmt.Errorf("unknown websocket frame type: %d", msgType)
			return
		}

		fmt.Fprint(s.rl.Stdout(), rxSprintf("< %s\n", text))
	}
}
