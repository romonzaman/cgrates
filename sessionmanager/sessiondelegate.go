/*
Rating system designed to be used in VoIP Carriers World
Copyright (C) 2012  Radu Ioan Fericean

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>
*/

package sessionmanager

import (
	"github.com/rif/cgrates/timespans"
	"log"
	"net/rpc"
	"net/rpc/jsonrpc"
	"time"
)

const (
	DEBIT_PERIOD = 10 * time.Second
)

var (
	// sample storage for the provided direct implementation
	storageGetter, _ = timespans.NewRedisStorage("tcp:127.0.0.1:6379", 10)
)

// Interface for the session delegate objects
type SessionDelegate interface {
	// Called on freeswitch's hearbeat event
	OnHeartBeat(Event)
	// Called on freeswitch's answer event
	OnChannelAnswer(Event, *Session)
	// Called on freeswitch's hangup event
	OnChannelHangupComplete(Event, *Session)
	// The method to be called inside the debit loop
	LoopAction(*Session, *timespans.CallDescriptor)
	// Returns a storage getter for the sesssion to use
	GetDebitPeriod() time.Duration
}

// Sample SessionDelegate calling the timespans methods directly
type DirectSessionDelegate byte

func (dsd *DirectSessionDelegate) OnHeartBeat(ev Event) {
	log.Print("♥")
}

func (dsd *DirectSessionDelegate) OnChannelAnswer(ev Event, s *Session) {
	log.Print("direct answer")
}

func (dsd *DirectSessionDelegate) OnChannelHangupComplete(ev Event, s *Session) {
	lastCC := s.CallCosts[len(s.CallCosts)-1]
	// put credit back	
	start := time.Now()
	end := lastCC.Timespans[len(lastCC.Timespans)-1].TimeEnd
	refoundDuration := end.Sub(start).Seconds()
	cost := 0.0
	seconds := 0.0
	log.Printf("Refund duration: %v", refoundDuration)
	for i := len(lastCC.Timespans) - 1; i >= 0; i-- {
		ts := lastCC.Timespans[i]
		tsDuration := ts.GetDuration().Seconds()
		if refoundDuration <= tsDuration {
			// find procentage
			procentage := (refoundDuration * 100) / tsDuration
			tmpCost := (procentage * ts.Cost) / 100
			ts.Cost -= tmpCost
			cost += tmpCost
			if ts.MinuteInfo != nil {
				// DestinationPrefix and Price take from lastCC and above caclulus
				seconds += (procentage * ts.MinuteInfo.Quantity) / 100
			}
			// set the end time to now
			ts.TimeEnd = start
			break // do not go to other timespans
		} else {
			cost += ts.Cost
			if ts.MinuteInfo != nil {
				seconds += ts.MinuteInfo.Quantity
			}
			// remove the timestamp entirely
			lastCC.Timespans = lastCC.Timespans[:i]
			// continue to the next timespan with what is left to refound
			refoundDuration -= tsDuration
		}
	}
	if cost > 0 {
		cd := &timespans.CallDescriptor{TOR: lastCC.TOR,
			CstmId:            lastCC.CstmId,
			Subject:           lastCC.CstmId,
			DestinationPrefix: lastCC.DestinationPrefix,
			Amount:            -cost,
		}
		cd.SetStorageGetter(storageGetter)
		cd.DebitCents()
	}
	if seconds > 0 {
		cd := &timespans.CallDescriptor{TOR: lastCC.TOR,
			CstmId:            lastCC.CstmId,
			Subject:           lastCC.CstmId,
			DestinationPrefix: lastCC.DestinationPrefix,
			Amount:            -seconds,
		}
		cd.SetStorageGetter(storageGetter)
		cd.DebitSeconds()
	}
	lastCC.Cost -= cost
	log.Printf("Rambursed %v cents, %v seconds", cost, seconds)
}

func (dsd *DirectSessionDelegate) LoopAction(s *Session, cd *timespans.CallDescriptor) {
	cd.SetStorageGetter(storageGetter)
	cc, err := cd.Debit()
	if err != nil {
		log.Printf("Could not complete debit opperation: %v", err)
	}
	s.CallCosts = append(s.CallCosts, cc)
	log.Print(cc)
	cd.Amount = DEBIT_PERIOD.Seconds()
	remainingSeconds, err := cd.GetMaxSessionTime()
	if remainingSeconds == -1 && err == nil {
		log.Print("Postpaying client: happy talking!")
		return
	}
	if remainingSeconds == 0 || err != nil {
		log.Printf("No credit left: Disconnect %v", s)
		s.Disconnect()
		return
	}
	if remainingSeconds < DEBIT_PERIOD.Seconds() || err != nil {
		log.Printf("Not enough money for another debit period %v", s)
		s.Disconnect()
		return
	}
}

func (dsd *DirectSessionDelegate) GetDebitPeriod() time.Duration {
	return DEBIT_PERIOD
}

// Sample SessionDelegate calling the timespans methods through the RPC interface
type RPCSessionDelegate struct {
	client *rpc.Client
}

func NewRPCSessionDelegate(host string) (rpc *RPCSessionDelegate) {
	client, err := jsonrpc.Dial("tcp", host)
	if err != nil {
		log.Fatalf("Could not connect to rater server %v!", err)
	}
	return &RPCSessionDelegate{client}
}

func (rsd *RPCSessionDelegate) OnHeartBeat(ev Event) {
	log.Print("rpc hearbeat")
}

func (rsd *RPCSessionDelegate) OnChannelAnswer(ev Event, s *Session, sm SessionManager) {
	log.Print("rpc answer")
}

func (rsd *RPCSessionDelegate) OnChannelHangupComplete(ev Event, s *Session) {
	log.Print("rpc hangup")
}

func (rsd *RPCSessionDelegate) LoopAction(s *Session, cd *timespans.CallDescriptor) {
	cc := &timespans.CallCost{}
	err := rsd.client.Call("Responder.Debit", cd, cc)
	if err != nil {
		log.Printf("Could not complete debit opperation: %v", err)
	}
	s.CallCosts = append(s.CallCosts, cc)
	log.Print(cc)
	cd.Amount = DEBIT_PERIOD.Seconds()
	var remainingSeconds float64
	err = rsd.client.Call("Responder.GetMaxSessionTime", cd, &remainingSeconds)
	if err != nil {
		log.Printf("Could not get max session time: %v", err)
	}
	if remainingSeconds == -1 && err == nil {
		log.Print("Postpaying client: happy talking!")
		return
	}
	if remainingSeconds == 0 || err != nil {
		log.Printf("No credit left: Disconnect %v", s)
		s.Disconnect()
		return
	}
	if remainingSeconds < DEBIT_PERIOD.Seconds() || err != nil {
		log.Printf("Not enough money for another debit period %v", s)
		s.Disconnect()
		return
	}
}
func (rsd *RPCSessionDelegate) GetDebitPeriod() time.Duration {
	return DEBIT_PERIOD
}