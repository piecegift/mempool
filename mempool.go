package mempool

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/btcsuite/btcutil"
	"github.com/patrickmn/go-cache"
	"golang.org/x/net/context/ctxhttp"
)

type Handler = func(ctx context.Context, txid, address string, amount btcutil.Amount)

func Start(ctx context.Context, testnet bool, handler Handler) {

	baseURL := "https://blockstream.info/api"
	if testnet {
		baseURL = "https://blockstream.info/testnet/api"
	}

	txidCache := cache.New(time.Hour, 2*time.Hour)

	for {
		timer := time.NewTimer(10 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}

		if err := iteration(ctx, baseURL, handler, txidCache); err != nil {
			log.Printf("Error when getting mempool: %v.", err)
		}
	}
}

func iteration(ctx context.Context, baseURL string, handler Handler, txidCache *cache.Cache) error {
	txidsURL := baseURL + "/mempool/txids"

	res, err := ctxhttp.Get(ctx, nil, txidsURL)
	if err != nil {
		return fmt.Errorf("failed to GET %s: %v", txidsURL, err)
	}

	var txids []string
	err = json.NewDecoder(res.Body).Decode(&txids)
	res.Body.Close()
	if err != nil {
		return fmt.Errorf("failed to parse %s's response: %v", txidsURL, err)
	}

	for _, txid := range txids {
		time.Sleep(100 * time.Millisecond)

		shortTxid, err := hex.DecodeString(txid)
		if err != nil {
			return fmt.Errorf("txid %s is not a valid hex: %v", txid, err)
		}

		if _, has := txidCache.Get(string(shortTxid)); has {
			continue
		}

		txURL := baseURL + "/tx/" + txid

		res, err := ctxhttp.Get(ctx, nil, txURL)
		if err != nil {
			return fmt.Errorf("failed to GET %s: %v", txURL, err)
		}

		var tx tx
		err = json.NewDecoder(res.Body).Decode(&tx)
		res.Body.Close()
		if err != nil {
			return fmt.Errorf("failed to parse tx %s: %v", txid, err)
		}

		for _, vout := range tx.Vouts {
			handler(ctx, txid, vout.Address, vout.Amount)
		}

		if err := txidCache.Add(string(shortTxid), 1, cache.DefaultExpiration); err != nil {
			panic(err)
		}
	}

	return nil
}

type tx struct {
	Vouts []vout `json:"vout"`
}

type vout struct {
	Address string         `json:"scriptpubkey_address"`
	Amount  btcutil.Amount `json:"value"`
}
