package api

import (
	"math/rand"
	"net/http"
	"reflect"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/rancher/go-rancher/api"
	"github.com/rancher/go-rancher/client"
	"github.com/sirupsen/logrus"

	"github.com/longhorn/longhorn-manager/controller"
)

const (
	keepAlivePeriod = 15 * time.Second

	writeWait = 10 * time.Second
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func NewStreamHandlerFunc(streamType string, watcher *controller.Watcher, listFunc func(ctx *api.ApiContext) (*client.GenericCollection, error)) func(w http.ResponseWriter, r *http.Request) error {
	return func(w http.ResponseWriter, r *http.Request) error {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return err
		}
		fields := logrus.Fields{
			"id":   strconv.Itoa(rand.Int()),
			"type": streamType,
		}
		logrus.WithFields(fields).Debug("websocket: open")

		done := make(chan struct{})
		go func() {
			defer close(done)
			for {
				_, _, err := conn.ReadMessage()
				if err != nil {
					logrus.WithFields(fields).Debug(err.Error())
					return
				}
			}
		}()

		apiContext := api.GetApiContext(r)

		resp, err := writeList(conn, nil, listFunc, apiContext)
		if err != nil {
			return err
		}

		rateLimitTicker := maybeNewTicker(getPeriod(r))
		if rateLimitTicker != nil {
			defer rateLimitTicker.Stop()
		}
		keepAliveTicker := time.NewTicker(keepAlivePeriod)
		defer keepAliveTicker.Stop()
		recentWrite := false
		for {
			if rateLimitTicker != nil {
				<-rateLimitTicker.C
			}
			select {
			case <-done:
				return nil
			case <-watcher.Events():
				resp, err = writeList(conn, resp, listFunc, apiContext)
				recentWrite = true
			case <-keepAliveTicker.C:
				err = conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(writeWait))
				if !recentWrite {
					resp, err = writeList(conn, nil, listFunc, apiContext)
				}
				recentWrite = !recentWrite
			}
			if err != nil {
				return err
			}
		}
	}
}

func writeList(conn *websocket.Conn, oldResp *client.GenericCollection, listFunc func(ctx *api.ApiContext) (*client.GenericCollection, error), apiContext *api.ApiContext) (*client.GenericCollection, error) {
	newResp, err := listFunc(apiContext)
	if err != nil {
		return oldResp, err
	}

	resp := newResp
	if oldResp != nil && reflect.DeepEqual(oldResp, newResp) {
		resp = &client.GenericCollection{}
	}
	data, err := apiContext.PopulateCollection(resp)
	if err != nil {
		return oldResp, err
	}

	conn.SetWriteDeadline(time.Now().Add(writeWait))
	err = conn.WriteJSON(data)
	if err != nil {
		return oldResp, err
	}

	if resp == newResp {
		return newResp, nil
	}
	return oldResp, nil
}

func maybeNewTicker(d time.Duration) *time.Ticker {
	var ticker *time.Ticker
	if d > 0*time.Second {
		ticker = time.NewTicker(d)
	}
	return ticker
}

func getPeriod(r *http.Request) time.Duration {
	period := 0 * time.Second
	periodString := mux.Vars(r)["period"]
	if periodString != "" {
		period, _ = time.ParseDuration(periodString)
	}
	switch {
	case period < 0*time.Second:
		period = 0 * time.Second
	case period > 15*time.Second:
		period = 15 * time.Second
	}
	return period
}
