// Copyright 2014 liveease.com. All rights reserved.

package signal

import (
	"code.google.com/p/go.net/websocket"
	"saassoft.net/signaldistribution/base"
	"time"
)

// Relay represents a relay client connecting to a remote station,it is kind of participant.
//
// The relay client listens to local station and remote station,
// when one of the local station's channels has produced a signal,
// it relays the signal to the remote station;
// when the relay client has received a signal from remote station,
// it relays the signal to the channel that signal belongs to in the local station.
type Relay struct {
	Info          ParticipantStruct
	Station       *Station
	RemoteSID     string
	RemoteMode    int
	RemoteIsTrunk bool
	Time          time.Time
	IsRequester   bool
}

// StartBroadcast starts to wait for signals from remote station, once a signal is received,
// the relay client broadcasts the signal to the station.
func (this *Relay) StartBroadcast() {
	for {
		var signal SignalPack
		if err := websocket.JSON.Receive(this.Info.Remote.Conn, &signal); err != nil {
			return
		}
		this.Relay(&signal)
	}
}

// StartListen starts to listen the station, once a signal is received, sends the signal to remote stations.
func (this *Relay) StartListen() {
	for signal := range this.Info.Signals {
		if signal == nil {
			return
		}
		if !base.StringInArray(this.RemoteSID, signal.Stations) {
			if err := websocket.JSON.Send(this.Info.Remote.Conn, signal); err != nil {
				break
			}
		}
	}
}

// Relay relays a signal to the station.
func (this *Relay) Relay(signal *SignalPack) error {
	addr := this.Station.Info.Addr()
	if !base.StringInArray(addr, signal.Stations) {
		this.Station.Broadcast(signal)
	}
	return nil
}

// PushSignal pushes a signal to the relay client.
func (this *Relay) PushSignal(signal *SignalPack) error {
	this.Info.Signals <- signal
	return nil
}

// PushSignal pushes signals to the relay client.
func (this *Relay) PushSignals(signals []*SignalPack) {
	for _, signal := range signals {
		this.PushSignal(signal)
	}
}

// Close closes the relay client,and release the resources of the relay client.
func (this *Relay) Release() {
	close(this.Info.Signals)
	_ = this.Info.Remote.Conn.Close()
}
