// Copyright 2014 liveease.com. All rights reserved.

package signal

import (
	"code.google.com/p/go-uuid/uuid"
	"code.google.com/p/go.net/websocket"
	"log"
	"saassoft.net/signaldistribution/base"
	"time"
)

// Client represents an end-client is connecting to a channel, it is kind of participant.
type Client struct {
	Info    ParticipantStruct
	Channel *Channel
}

// StartBroadcast starts to wait for producing signals, once a new signal is produced,
// the client broadcasts the signal to the station.
func (this *Client) StartBroadcast() {
	for {
		var signal Signal
		if err := websocket.JSON.Receive(this.Info.Remote.Conn, &signal); err != nil {
			return
		}
		if signal.Type == base.SIGNALTYPE_BLANK {
			continue
		}
		signal.ID = uuid.New()
		signal.PID = this.Info.PID
		signalPack := SignalPack{
			Signal:   signal,
			CID:      this.Channel.CID,
			Time:     time.Now(),
			Stations: []string{},
		}
		log.Println("station - client: new signal:", signal.Text)
		this.Relay(&signalPack)
	}
}

// StartListen starts to listen the station, once a signal is received, sends the signal to the client.
func (this *Client) StartListen() {
	for b := range this.Info.Signals {
		if b == nil {
			return
		}
		err := websocket.JSON.Send(this.Info.Remote.Conn, b.Signal)
		if err != nil {
			break
		}
	}
}

// Relay relays a signal to the channel listening to.
func (this *Client) Relay(signal *SignalPack) error {
	return this.Channel.Broadcast(signal)
}

// PushSignal pushes a signal to the client.
func (this *Client) PushSignal(signal *SignalPack) error {
	this.Info.Signals <- signal
	return nil
}

// PushSignal pushes signals to the client.
func (this *Client) PushSignals(signals []*SignalPack) {
	for _, signal := range signals {
		this.PushSignal(signal)
	}
}

// Close closes the client,and release the resources of the client.
func (this *Client) Close() {
	close(this.Info.Signals)
	_ = this.Info.Remote.Conn.Close()
}
