package service

import (
	"context"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"

	"paymentg/internal/database"
)

type Poller struct {
	db                 *database.DB
	shopeeClient       *ShopeeClient
	matcher            *Matcher
	interval           time.Duration
	tokenValid         bool
	tokenError         string
	lastPollAt         time.Time
	lastPollOK         bool
	lastRefreshAttempt time.Time
	refreshing         bool
	mu                 sync.RWMutex
}

func NewPoller(db *database.DB, shopeeClient *ShopeeClient, matcher *Matcher, intervalSec int) *Poller {
	return &Poller{
		db:           db,
		shopeeClient: shopeeClient,
		matcher:      matcher,
		interval:     time.Duration(intervalSec) * time.Second,
	}
}

func (p *Poller) Start(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			count, err := p.db.CountPendingOrders()
			if err != nil {
				log.Printf("Poller error counting pending orders: %v", err)
				continue
			}
			if count == 0 {
				log.Printf("Debug: no pending orders, skipping poll")
				continue
			}

			token, _ := p.db.GetConfig("shopee_token")
			if token == "" {
				p.SetTokenValid(false, "token empty")
				p.triggerTokenRefresh()
				continue
			}

			transactions, err := p.shopeeClient.FetchTransactions(token)
			p.mu.Lock()
			if err != nil {
				p.tokenValid = false
				p.tokenError = err.Error()
				p.lastPollOK = false
				p.mu.Unlock()
				log.Printf("Poller fetch error: %v", err)
				p.triggerTokenRefresh()
				continue
			}

			p.tokenValid = true
			p.tokenError = ""
			p.lastPollAt = time.Now()
			p.lastPollOK = true
			p.mu.Unlock()

			matches, err := p.matcher.MatchTransactions(transactions)
			if err != nil {
				log.Printf("Poller match error: %v", err)
			} else if matches > 0 {
				log.Printf("Info: matched %d transactions", matches)
			}
		}
	}
}

func (p *Poller) triggerTokenRefresh() {
	p.mu.Lock()
	if p.refreshing || time.Since(p.lastRefreshAttempt) < 3*time.Minute {
		p.mu.Unlock()
		return
	}
	p.refreshing = true
	p.lastRefreshAttempt = time.Now()
	p.mu.Unlock()

	go func() {
		defer func() {
			p.mu.Lock()
			p.refreshing = false
			p.mu.Unlock()
		}()

		if _, err := os.Stat("refresh_token.py"); err != nil {
			return
		}

		log.Println("[Poller] Token kadaluwarsa/kosong. Menjalankan refresh_token.py (Playwright)...")
		cmd := exec.Command("python", "refresh_token.py")
		out, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("[Poller] refresh_token.py gagal: %v, output: %s", err, string(out))
		} else {
			log.Printf("[Poller] refresh_token.py selesai: %s", string(out))
		}
	}()
}

func (p *Poller) Status() (bool, string, time.Time, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.tokenValid, p.tokenError, p.lastPollAt, p.lastPollOK
}

func (p *Poller) SetTokenValid(valid bool, errMsg string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.tokenValid = valid
	p.tokenError = errMsg
}
