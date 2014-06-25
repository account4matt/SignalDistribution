// Copyright 2014 liveease.com. All rights reserved.

// Package route implements route server and route client.
package route

import (
	"saassoft.net/signaldistribution/base"
	"strings"
	"time"
)

// Nats stores the nat addresses.
var Nats map[string]string

// Recorder represents the base information of a recorder server.
type Recorder struct {
	UPID        string
	PublishAddr string
	Time        time.Time
}

// Station represents the base information of a station server.
type Station struct {
	SID         string
	Mode        base.StationMode
	RemoteInfo  base.RemoteInfo
	PublishAddr string
	Time        time.Time
	Clients     map[string]time.Time
	Recorders   map[string]*Recorder
	IsOnline    bool
	TrunkRelays map[string]*Relay
	Relays      map[string]*Relay
}

// RemoveRecorder removes the relationship between a recorder and the station.
func (this *Station) RemoveRecorder(upid string) {
	delete(this.Recorders, upid)
}

// AppendRecorder appends a relationship between a recorder and the station.
func (this *Station) AppendRecorder(upid string) {
	if upid == "" {
		return
	}
	ips := strings.Split(upid, "-")
	if len(ips) < 2 {
		return
	}
	publishAddr := Nats[ips[1]]
	if publishAddr == "" {
		publishAddr = ips[1]
	}
	this.Recorders[upid] = &Recorder{
		UPID:        upid,
		PublishAddr: publishAddr,
		Time:        time.Now(),
	}
}

// RemoveClient removes the relationship between a client and the station.
func (this *Station) RemoveClient(upid string) {
	delete(this.Clients, upid)
}

// AppendClient appends a relationship between a client and the station.
func (this *Station) AppendClient(upid string) {
	this.Clients[upid] = time.Now()
}

// RemoveTrunkRelay removes the relationship between a trunk station and the station when it runs as trunk mode.
func (this *Station) RemoveTrunkRelay(upid string) {
	if this.TrunkRelays[upid] != nil {
		delete(this.TrunkRelays, upid)
	}
}

// AppendTrunkRelay appends a relationship between a trunk station and the station when it runs as trunk mode.
func (this *Station) AppendTrunkRelay(upid string, relay *Relay) {
	this.TrunkRelays[upid] = relay
}

// ExistsTrunkRelay returns true if the station exists the relationship with a trunk station.
func (this *Station) ExistsTrunkRelay(upid string) bool {
	return this.TrunkRelays[upid] != nil
}

// RemoveRelay removes the relationship between a relay and the station.
func (this *Station) RemoveRelay(upid string) {
	if this.Relays[upid] != nil {
		delete(this.Relays, upid)
	}
}

// AppendRelay appends a relationship between a relay and the station.
func (this *Station) AppendRelay(upid string, relay *Relay) {
	if relay.IsTrunk {
		this.TrunkRelays[upid] = relay
	} else {
		this.Relays[upid] = relay
	}
}

// ExistsRelay returns true if the station exists the relationship with a relay.
func (this *Station) ExistsRelay(upid string) bool {
	return this.Relays[upid] != nil
}

// Relay represents the base information of a relay between two stations.
type Relay struct {
	FromSID string
	ToSID   string
	IsTrunk bool
	Time    time.Time
}
