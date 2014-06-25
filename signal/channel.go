// Copyright 2014 liveease.com. All rights reserved.

package signal

import (
	"sync"
)

// Channel is the channel for transmitting the signals.
type Channel struct {
	CID                    string
	Station                *Station
	BeforeBroadcastHandler func(*Channel, *SignalPack) bool
	AfterBroadcastHandler  func(*Channel, *SignalPack) bool
	closeSign              chan bool
	isClosed               bool
	closeLock              sync.Mutex
	clients                map[string]*Client
	broadcast              chan *SignalPack
}

// Init sets up the channel.
func (this *Channel) Init() {
	this.closeSign = make(chan bool)
	this.clients = make(map[string]*Client)
	this.broadcast = make(chan *SignalPack)
}

// Init sets up the channel with cid and station.
func (this *Channel) InitWith(cid string, station *Station) {
	this.Init()
	this.CID = cid
	this.Station = station
}

// Run makes the channel start to listen the signals, once the channel has received a signal,
// relays the signal to other stations relay-connected with the station and sends the signal to the clients listening the channel,
// if there are recorders, the channel records the signal to the recorders also.
func (this *Channel) Run() {
	for {
		select {
		case signal := <-this.broadcast:
			go this.Station.RecordSignal(signal)
			go this.Station.RelayToRemoteStations(signal)
			go this.sendToClients(signal)
		case closed := <-this.closeSign:
			if closed == true {
				this.release()
				return
			}
		}
	}
}

// ClientCount returns the count of clients connected.
func (this *Channel) ClientCount() int {
	return len(this.clients)
}

// ClientJoin sets up the client that joins in the channel.
func (this *Channel) ClientJoin(client *Client) {
	this.clients[client.Info.UPID] = client
}

// ClientQuit deletes the client when it quits the channel.
func (this *Channel) ClientQuit(client *Client) {
	this.clients[client.Info.UPID] = nil
	delete(this.clients, client.Info.UPID)
}

// GetClientByUPID returns the client with the upid in the channel.
func (this *Channel) GetClientByUPID(upid string) *Client {
	return this.clients[upid]
}

// Clients returns the clients in the channel.
func (this *Channel) Clients() map[string]*Client {
	return this.clients
}

// Broadcast broadcasts the signal to the channel.
func (this *Channel) Broadcast(signal *SignalPack) error {
	if this.BeforeBroadcastHandler == nil || this.BeforeBroadcastHandler(this, signal) {
		this.broadcast <- signal
		if this.AfterBroadcastHandler != nil {
			this.AfterBroadcastHandler(this, signal)
		}
	}
	return nil
}

// Close closes the channel,clean the resources of the channel.
func (this *Channel) Close() {
	this.closeLock.Lock()
	defer this.closeLock.Unlock()
	if this.isClosed {
		return
	}
	this.isClosed = true
	this.closeSign <- true
}

func (this *Channel) sendToClients(signal *SignalPack) {
	for _, client := range this.clients {
		client.PushSignal(signal)
	}
}

func (this *Channel) release() {
	close(this.broadcast)
	close(this.closeSign)
}
