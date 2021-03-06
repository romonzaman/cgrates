/*
Rating system designed to be used in VoIP Carriers World
Copyright (C) 2012-2015 ITsysCOM GmbH

This program is free software: you can Storagetribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITH*out ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>
*/

package general_tests

import (
	"testing"
	"time"

	"github.com/cgrates/cgrates/cache2go"
	"github.com/cgrates/cgrates/engine"
	"github.com/cgrates/cgrates/scheduler"
	"github.com/cgrates/cgrates/utils"
)

var ratingDb3 engine.RatingStorage
var acntDb3 engine.AccountingStorage

func TestSetStorage3(t *testing.T) {
	ratingDb3, _ = engine.NewMapStorageJson()
	engine.SetRatingStorage(ratingDb3)
	acntDb3, _ = engine.NewMapStorageJson()
	engine.SetAccountingStorage(acntDb3)
}

func TestLoadCsvTp3(t *testing.T) {
	timings := `ALWAYS,*any,*any,*any,*any,00:00:00
ASAP,*any,*any,*any,*any,*asap`
	destinations := `DST_UK_Mobile_BIG5,447596
DST_UK_Mobile_BIG5,447956`
	rates := `RT_UK_Mobile_BIG5_PKG,0.01,0,20s,20s,0s
RT_UK_Mobile_BIG5,0.01,0.10,1s,1s,0s`
	destinationRates := `DR_UK_Mobile_BIG5_PKG,DST_UK_Mobile_BIG5,RT_UK_Mobile_BIG5_PKG,*up,8,0,
DR_UK_Mobile_BIG5,DST_UK_Mobile_BIG5,RT_UK_Mobile_BIG5,*up,8,0,`
	ratingPlans := `RP_UK_Mobile_BIG5_PKG,DR_UK_Mobile_BIG5_PKG,ALWAYS,10
RP_UK,DR_UK_Mobile_BIG5,ALWAYS,10`
	ratingProfiles := `*out,cgrates.org,call,*any,2013-01-06T00:00:00Z,RP_UK,,
*out,cgrates.org,call,discounted_minutes,2013-01-06T00:00:00Z,RP_UK_Mobile_BIG5_PKG,,`
	sharedGroups := ``
	lcrs := ``
	actions := `TOPUP10_AC1,*topup_reset,,,*voice,*out,,DST_UK_Mobile_BIG5,discounted_minutes,,*unlimited,,40,10,10`
	actionPlans := `TOPUP10_AT,TOPUP10_AC1,ASAP,10`
	actionTriggers := ``
	accountActions := `cgrates.org,12346,*out,TOPUP10_AT,`
	derivedCharges := ``
	cdrStats := ``
	csvr := engine.NewTpReader(ratingDb3, acntDb3, engine.NewStringCSVStorage(',', destinations, timings, rates, destinationRates, ratingPlans, ratingProfiles,
		sharedGroups, lcrs, actions, actionPlans, actionTriggers, accountActions, derivedCharges, cdrStats), "")
	if err := csvr.LoadDestinations(); err != nil {
		t.Fatal(err)
	}
	if err := csvr.LoadTimings(); err != nil {
		t.Fatal(err)
	}
	if err := csvr.LoadRates(); err != nil {
		t.Fatal(err)
	}
	if err := csvr.LoadDestinationRates(); err != nil {
		t.Fatal(err)
	}
	if err := csvr.LoadRatingPlans(); err != nil {
		t.Fatal(err)
	}
	if err := csvr.LoadRatingProfiles(); err != nil {
		t.Fatal(err)
	}
	if err := csvr.LoadSharedGroups(); err != nil {
		t.Fatal(err)
	}
	if err := csvr.LoadLCRs(); err != nil {
		t.Fatal(err)
	}
	if err := csvr.LoadActions(); err != nil {
		t.Fatal(err)
	}
	if err := csvr.LoadActionPlans(); err != nil {
		t.Fatal(err)
	}
	if err := csvr.LoadActionTriggers(); err != nil {
		t.Fatal(err)
	}
	if err := csvr.LoadAccountActions(); err != nil {
		t.Fatal(err)
	}
	if err := csvr.LoadDerivedChargers(); err != nil {
		t.Fatal(err)
	}
	csvr.WriteToDatabase(false, false)
	if acnt, err := acntDb3.GetAccount("*out:cgrates.org:12346"); err != nil {
		t.Error(err)
	} else if acnt == nil {
		t.Error("No account saved")
	}
	ratingDb3.CacheRating(nil, nil, nil, nil, nil, nil)
	acntDb3.CacheAccounting(nil, nil, nil)
	if cachedDests := cache2go.CountEntries(engine.DESTINATION_PREFIX); cachedDests != 2 {
		t.Error("Wrong number of cached destinations found", cachedDests)
	}
	if cachedRPlans := cache2go.CountEntries(engine.RATING_PLAN_PREFIX); cachedRPlans != 2 {
		t.Error("Wrong number of cached rating plans found", cachedRPlans)
	}
	if cachedRProfiles := cache2go.CountEntries(engine.RATING_PROFILE_PREFIX); cachedRProfiles != 2 {
		t.Error("Wrong number of cached rating profiles found", cachedRProfiles)
	}
	if cachedActions := cache2go.CountEntries(engine.ACTION_PREFIX); cachedActions != 1 {
		t.Error("Wrong number of cached actions found", cachedActions)
	}
}

func TestExecuteActions3(t *testing.T) {
	scheduler.NewScheduler().LoadActionPlans(acntDb3)
	time.Sleep(time.Millisecond) // Give time to scheduler to topup the account
	if acnt, err := acntDb3.GetAccount("*out:cgrates.org:12346"); err != nil {
		t.Error(err)
	} else if len(acnt.BalanceMap) != 1 {
		t.Error("Account does not have enough balances: ", acnt.BalanceMap)
	} else if acnt.BalanceMap[utils.VOICE+engine.OUTBOUND][0].Value != 40 {
		t.Error("Account does not have enough minutes in balance", acnt.BalanceMap[utils.VOICE+engine.OUTBOUND][0].Value)
	}
}

func TestDebit3(t *testing.T) {
	cd := &engine.CallDescriptor{
		Direction:   "*out",
		Category:    "call",
		Tenant:      "cgrates.org",
		Subject:     "12346",
		Account:     "12346",
		Destination: "447956933443",
		TimeStart:   time.Date(2014, 3, 4, 6, 0, 0, 0, time.UTC),
		TimeEnd:     time.Date(2014, 3, 4, 6, 0, 10, 0, time.UTC),
	}
	if cc, err := cd.Debit(); err != nil {
		t.Error(err)
	} else if cc.Cost != 0.01 {
		t.Error("Wrong cost returned: ", cc.Cost)
	}
	acnt, err := acntDb3.GetAccount("*out:cgrates.org:12346")
	if err != nil {
		t.Error(err)
	}
	if len(acnt.BalanceMap) != 2 {
		t.Error("Wrong number of user balances found", acnt.BalanceMap)
	}
	if acnt.BalanceMap[utils.VOICE+engine.OUTBOUND][0].Value != 20 {
		t.Error("Account does not have expected minutes in balance", acnt.BalanceMap[utils.VOICE+engine.OUTBOUND][0].Value)
	}
	if acnt.BalanceMap[utils.MONETARY+engine.OUTBOUND][0].Value != -0.01 {
		t.Error("Account does not have expected monetary balance", acnt.BalanceMap[utils.MONETARY+engine.OUTBOUND][0].Value)
	}
}
