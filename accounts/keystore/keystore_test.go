// Modifications Copyright 2018 The klaytn Authors
// Copyright 2017 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.
//
// This file is derived from accounts/keystore/keystore_test.go (2018/06/04).
// Modified and improved for the klaytn development.

package keystore

import (
	"crypto/ecdsa"
	"io/ioutil"
	"math/big"
	"math/rand"
	"os"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/klaytn/klaytn/blockchain/types"
	"github.com/klaytn/klaytn/crypto"
	"github.com/klaytn/klaytn/params"
	"github.com/stretchr/testify/assert"

	"github.com/klaytn/klaytn/accounts"
	"github.com/klaytn/klaytn/common"
	"github.com/klaytn/klaytn/event"
)

var testSigData = make([]byte, 32)

func TestKeyStore(t *testing.T) {
	dir, ks := tmpKeyStore(t, true)
	defer os.RemoveAll(dir)

	a, err := ks.NewAccount("foo")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(a.URL.Path, dir) {
		t.Errorf("account file %s doesn't have dir prefix", a.URL)
	}
	stat, err := os.Stat(a.URL.Path)
	if err != nil {
		t.Fatalf("account file %s doesn't exist (%v)", a.URL, err)
	}
	if runtime.GOOS != "windows" && stat.Mode() != 0600 {
		t.Fatalf("account file has wrong mode: got %o, want %o", stat.Mode(), 0600)
	}
	if !ks.HasAddress(a.Address) {
		t.Errorf("HasAccount(%x) should've returned true", a.Address)
	}
	if err := ks.Update(a, "foo", "bar"); err != nil {
		t.Errorf("Update error: %v", err)
	}
	if err := ks.Delete(a, "bar"); err != nil {
		t.Errorf("Delete error: %v", err)
	}
	if common.FileExist(a.URL.Path) {
		t.Errorf("account file %s should be gone after Delete", a.URL)
	}
	if ks.HasAddress(a.Address) {
		t.Errorf("HasAccount(%x) should've returned true after Delete", a.Address)
	}
}

func TestSign(t *testing.T) {
	dir, ks := tmpKeyStore(t, true)
	defer os.RemoveAll(dir)

	pass := "" // not used but required by API
	a1, err := ks.NewAccount(pass)
	if err != nil {
		t.Fatal(err)
	}
	if err := ks.Unlock(a1, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := ks.SignHash(accounts.Account{Address: a1.Address}, testSigData); err != nil {
		t.Fatal(err)
	}
}

func TestSignWithPassphrase(t *testing.T) {
	dir, ks := tmpKeyStore(t, true)
	defer os.RemoveAll(dir)

	pass := "passwd"
	acc, err := ks.NewAccount(pass)
	if err != nil {
		t.Fatal(err)
	}

	if _, unlocked := ks.unlocked[acc.Address]; unlocked {
		t.Fatal("expected account to be locked")
	}

	_, err = ks.SignHashWithPassphrase(acc, pass, testSigData)
	if err != nil {
		t.Fatal(err)
	}

	if _, unlocked := ks.unlocked[acc.Address]; unlocked {
		t.Fatal("expected account to be locked")
	}

	if _, err = ks.SignHashWithPassphrase(acc, "invalid passwd", testSigData); err == nil {
		t.Fatal("expected SignHashWithPassphrase to fail with invalid password")
	}
}

func TestTimedUnlock(t *testing.T) {
	dir, ks := tmpKeyStore(t, true)
	defer os.RemoveAll(dir)

	pass := "foo"
	a1, err := ks.NewAccount(pass)
	if err != nil {
		t.Fatal(err)
	}

	// Signing without passphrase fails because account is locked
	_, err = ks.SignHash(accounts.Account{Address: a1.Address}, testSigData)
	if err != ErrLocked {
		t.Fatal("Signing should've failed with ErrLocked before unlocking, got ", err)
	}

	// Signing with passphrase works
	if err = ks.TimedUnlock(a1, pass, 100*time.Millisecond); err != nil {
		t.Fatal(err)
	}

	// Signing without passphrase works because account is temp unlocked
	_, err = ks.SignHash(accounts.Account{Address: a1.Address}, testSigData)
	if err != nil {
		t.Fatal("Signing shouldn't return an error after unlocking, got ", err)
	}

	// Signing fails again after automatic locking
	time.Sleep(250 * time.Millisecond)
	_, err = ks.SignHash(accounts.Account{Address: a1.Address}, testSigData)
	if err != ErrLocked {
		t.Fatal("Signing should've failed with ErrLocked timeout expired, got ", err)
	}
}

func TestOverrideUnlock(t *testing.T) {
	dir, ks := tmpKeyStore(t, false)
	defer os.RemoveAll(dir)

	pass := "foo"
	a1, err := ks.NewAccount(pass)
	if err != nil {
		t.Fatal(err)
	}

	// Unlock indefinitely.
	if err = ks.TimedUnlock(a1, pass, 5*time.Minute); err != nil {
		t.Fatal(err)
	}

	// Signing without passphrase works because account is temp unlocked
	_, err = ks.SignHash(accounts.Account{Address: a1.Address}, testSigData)
	if err != nil {
		t.Fatal("Signing shouldn't return an error after unlocking, got ", err)
	}

	// reset unlock to a shorter period, invalidates the previous unlock
	if err = ks.TimedUnlock(a1, pass, 100*time.Millisecond); err != nil {
		t.Fatal(err)
	}

	// Signing without passphrase still works because account is temp unlocked
	_, err = ks.SignHash(accounts.Account{Address: a1.Address}, testSigData)
	if err != nil {
		t.Fatal("Signing shouldn't return an error after unlocking, got ", err)
	}

	// Signing fails again after automatic locking
	time.Sleep(250 * time.Millisecond)
	_, err = ks.SignHash(accounts.Account{Address: a1.Address}, testSigData)
	if err != ErrLocked {
		t.Fatal("Signing should've failed with ErrLocked timeout expired, got ", err)
	}
}

// This test should fail under -race if signing races the expiration goroutine.
func TestSignRace(t *testing.T) {
	dir, ks := tmpKeyStore(t, false)
	defer os.RemoveAll(dir)

	// Create a test account.
	a1, err := ks.NewAccount("")
	if err != nil {
		t.Fatal("could not create the test account", err)
	}

	if err := ks.TimedUnlock(a1, "", 15*time.Millisecond); err != nil {
		t.Fatal("could not unlock the test account", err)
	}
	end := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(end) {
		if _, err := ks.SignHash(accounts.Account{Address: a1.Address}, testSigData); err == ErrLocked {
			return
		} else if err != nil {
			t.Errorf("Sign error: %v", err)
			return
		}
		time.Sleep(1 * time.Millisecond)
	}
	t.Errorf("Account did not lock within the timeout")
}

// Tests that the wallet notifier loop starts and stops correctly based on the
// addition and removal of wallet event subscriptions.
func TestWalletNotifierLifecycle(t *testing.T) {
	// Create a temporary kesytore to test with
	dir, ks := tmpKeyStore(t, false)
	defer os.RemoveAll(dir)

	// Ensure that the notification updater is not running yet
	time.Sleep(250 * time.Millisecond)
	ks.mu.RLock()
	updating := ks.updating
	ks.mu.RUnlock()

	if updating {
		t.Errorf("wallet notifier running without subscribers")
	}
	// Subscribe to the wallet feed and ensure the updater boots up
	updates := make(chan accounts.WalletEvent)

	subs := make([]event.Subscription, 2)
	for i := 0; i < len(subs); i++ {
		// Create a new subscription
		subs[i] = ks.Subscribe(updates)

		// Ensure the notifier comes online
		time.Sleep(250 * time.Millisecond)
		ks.mu.RLock()
		updating = ks.updating
		ks.mu.RUnlock()

		if !updating {
			t.Errorf("sub %d: wallet notifier not running after subscription", i)
		}
	}
	// Unsubscribe and ensure the updater terminates eventually
	for i := 0; i < len(subs); i++ {
		// Close an existing subscription
		subs[i].Unsubscribe()

		// Ensure the notifier shuts down at and only at the last close
		for k := 0; k < int(walletRefreshCycle/(250*time.Millisecond))+2; k++ {
			ks.mu.RLock()
			updating = ks.updating
			ks.mu.RUnlock()

			if i < len(subs)-1 && !updating {
				t.Fatalf("sub %d: event notifier stopped prematurely", i)
			}
			if i == len(subs)-1 && !updating {
				return
			}
			time.Sleep(250 * time.Millisecond)
		}
	}
	t.Errorf("wallet notifier didn't terminate after unsubscribe")
}

type walletEvent struct {
	accounts.WalletEvent
	a accounts.Account
}

// Tests that wallet notifications and correctly fired when accounts are added
// or deleted from the keystore.
func TestWalletNotifications(t *testing.T) {
	dir, ks := tmpKeyStore(t, false)
	defer os.RemoveAll(dir)

	// Subscribe to the wallet feed and collect events.
	var (
		events  []walletEvent
		updates = make(chan accounts.WalletEvent)
		sub     = ks.Subscribe(updates)
	)
	defer sub.Unsubscribe()
	go func() {
		for {
			select {
			case ev := <-updates:
				events = append(events, walletEvent{ev, ev.Wallet.Accounts()[0]})
			case <-sub.Err():
				close(updates)
				return
			}
		}
	}()

	// Randomly add and remove accounts.
	var (
		live       = make(map[common.Address]accounts.Account)
		wantEvents []walletEvent
	)
	for i := 0; i < 1024; i++ {
		if create := len(live) == 0 || rand.Int()%4 > 0; create {
			// Add a new account and ensure wallet notifications arrives
			account, err := ks.NewAccount("")
			if err != nil {
				t.Fatalf("failed to create test account: %v", err)
			}
			live[account.Address] = account
			wantEvents = append(wantEvents, walletEvent{accounts.WalletEvent{Kind: accounts.WalletArrived}, account})
		} else {
			// Delete a random account.
			var account accounts.Account
			for _, a := range live {
				account = a
				break
			}
			if err := ks.Delete(account, ""); err != nil {
				t.Fatalf("failed to delete test account: %v", err)
			}
			delete(live, account.Address)
			wantEvents = append(wantEvents, walletEvent{accounts.WalletEvent{Kind: accounts.WalletDropped}, account})
		}
	}

	// Shut down the event collector and check events.
	sub.Unsubscribe()
	<-updates
	checkAccounts(t, live, ks.Wallets())
	checkEvents(t, wantEvents, events)
}

// checkAccounts checks that all known live accounts are present in the wallet list.
func checkAccounts(t *testing.T, live map[common.Address]accounts.Account, wallets []accounts.Wallet) {
	if len(live) != len(wallets) {
		t.Errorf("wallet list doesn't match required accounts: have %d, want %d", len(wallets), len(live))
		return
	}
	liveList := make([]accounts.Account, 0, len(live))
	for _, account := range live {
		liveList = append(liveList, account)
	}
	sort.Sort(accountsByURL(liveList))
	for j, wallet := range wallets {
		if accs := wallet.Accounts(); len(accs) != 1 {
			t.Errorf("wallet %d: contains invalid number of accounts: have %d, want 1", j, len(accs))
		} else if accs[0] != liveList[j] {
			t.Errorf("wallet %d: account mismatch: have %v, want %v", j, accs[0], liveList[j])
		}
	}
}

// checkEvents checks that all events in 'want' are present in 'have'. Events may be present multiple times.
func checkEvents(t *testing.T, want []walletEvent, have []walletEvent) {
	for _, wantEv := range want {
		nmatch := 0
		for ; len(have) > 0; nmatch++ {
			if have[0].Kind != wantEv.Kind || have[0].a != wantEv.a {
				break
			}
			have = have[1:]
		}
		if nmatch == 0 {
			t.Fatalf("can't find event with Kind=%v for %x", wantEv.Kind, wantEv.a.Address)
		}
	}
}

func tmpKeyStore(t *testing.T, encrypted bool) (string, *KeyStore) {
	d, err := ioutil.TempDir("", "klay-keystore-test")
	if err != nil {
		t.Fatal(err)
	}
	newKs := NewPlaintextKeyStore
	if encrypted {
		newKs = func(kd string) *KeyStore { return NewKeyStore(kd, veryLightScryptN, veryLightScryptP) }
	}
	return d, newKs(string(d))
}

// testTx returns a sample transaction and private keys of the sender and fee payer.
func testTx() (*ecdsa.PrivateKey, *ecdsa.PrivateKey, *types.Transaction) {
	// For signers
	senderPrvKey, _ := crypto.HexToECDSA("95a21e86efa290d6665a9dbce06ae56319335540d13540fb1b01e28a5b2c8460")
	feePayerPrvKey, _ := crypto.HexToECDSA("f45a87856d14357609ce6b99645a1a7889deafbf00848bbdace60d8cd10466fa")

	values := map[types.TxValueKeyType]interface{}{
		types.TxValueKeyNonce:    uint64(0),
		types.TxValueKeyFrom:     common.HexToAddress("0xa7Eb6992c5FD55F43305B24Ee67150Bf4910d329"),
		types.TxValueKeyTo:       common.HexToAddress("0xF9Fad0E94B216faFFfEfB99Ef02CE44F994A3DE8"),
		types.TxValueKeyAmount:   new(big.Int).SetUint64(0),
		types.TxValueKeyGasLimit: uint64(100000),
		types.TxValueKeyGasPrice: new(big.Int).SetUint64(25 * params.Ston),
		types.TxValueKeyFeePayer: common.HexToAddress("0xF9Fad0E94B216faFFfEfB99Ef02CE44F994A3DE8"),
	}

	tx, err := types.NewTransactionWithMap(types.TxTypeFeeDelegatedValueTransfer, values)
	if err != nil {
		panic("Error to generate a test tx")
	}

	return senderPrvKey, feePayerPrvKey, tx
}

// TestKeyStore_SignTx tests the tx signing function of KeyStore.
func TestKeyStore_SignTx(t *testing.T) {
	chainID := big.NewInt(1)

	// test transaction and the private key of the sender
	senderPrvKey, _, tx := testTx()

	// generate a keystore and an active account
	dir, ks := tmpKeyStore(t, true)
	defer os.RemoveAll(dir)

	password := ""
	acc, err := ks.ImportECDSA(senderPrvKey, password)
	if err != nil {
		t.Fatal(err)
	}
	if err := ks.Unlock(acc, password); err != nil {
		t.Fatal(err)
	}

	// get a signature from a signing function in KeyStore
	tx, err = ks.SignTx(accounts.Account{Address: acc.Address}, tx, chainID)
	if err != nil {
		t.Fatal(err)
	}
	sig1 := tx.RawSignatureValues()

	// get another signature from a signing function in types package
	signer := types.LatestSignerForChainID(chainID)
	if tx.SignWithKeys(signer, []*ecdsa.PrivateKey{senderPrvKey}) != nil {
		t.Fatal("Error to sign")
	}
	sig2 := tx.RawSignatureValues()

	// Two signing functions should return the same signature
	assert.Equal(t, sig2, sig1)
}

// TestKeyStore_SignTxAsFeePayer tests the fee payer's tx signing function of KeyStore.
func TestKeyStore_SignTxAsFeePayer(t *testing.T) {
	chainID := big.NewInt(1)

	// test transaction and the private key of the sender
	_, feePayerPrvKey, tx := testTx()

	// generate a keystore and an active account
	dir, ks := tmpKeyStore(t, true)
	defer os.RemoveAll(dir)

	password := ""
	acc, err := ks.ImportECDSA(feePayerPrvKey, password)
	if err != nil {
		t.Fatal(err)
	}
	if err := ks.Unlock(acc, password); err != nil {
		t.Fatal(err)
	}

	// get a signature from a signing function in KeyStore
	tx, err = ks.SignTxAsFeePayer(accounts.Account{Address: acc.Address}, tx, chainID)
	if err != nil {
		t.Fatal(err)
	}
	sig1, err := tx.GetFeePayerSignatures()
	if err != nil {
		t.Fatal(err)
	}

	// get another signature from a signing function in types package
	signer := types.LatestSignerForChainID(chainID)
	if tx.SignFeePayerWithKeys(signer, []*ecdsa.PrivateKey{feePayerPrvKey}) != nil {
		t.Fatal("Error to sign")
	}
	sig2, err := tx.GetFeePayerSignatures()
	if err != nil {
		t.Fatal(err)
	}

	// Two signing functions should return the same value
	assert.Equal(t, sig2, sig1)
}
