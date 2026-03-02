package price

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"explorer/internal/events"
)

type PriceData struct {
	Price          float64   `json:"price"`
	Currency       string    `json:"currency"`
	Change24h      float64   `json:"change24h"`
	MarketCap      float64   `json:"marketCap,omitempty"`
	Volume24h      float64   `json:"volume24h,omitempty"`
	LastUpdated    time.Time `json:"lastUpdated"`
}

type Service struct {
	mu          sync.RWMutex
	cache       *PriceData
	cacheExpiry time.Time
	cacheTTL    time.Duration
	httpClient  *http.Client
	coinID      string // CoinGecko coin ID (e.g., "ethereum")
	currency    string // Fiat currency (e.g., "usd")
	eventBus    *events.Bus
}

func (s *Service) SetEventBus(bus *events.Bus) {
	s.eventBus = bus
}

type coingeckoResponse map[string]struct {
	USD          float64 `json:"usd"`
	USDChange24h float64 `json:"usd_24h_change"`
	USDMarketCap float64 `json:"usd_market_cap"`
	USDVolume24h float64 `json:"usd_24h_vol"`
}

func NewService(coinID, currency string, cacheTTL time.Duration) *Service {
	return &Service{
		cacheTTL:   cacheTTL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		coinID:     coinID,
		currency:   currency,
	}
}

func (s *Service) GetPrice(ctx context.Context) (*PriceData, error) {
	s.mu.RLock()
	if s.cache != nil && time.Now().Before(s.cacheExpiry) {
		defer s.mu.RUnlock()
		return s.cache, nil
	}
	s.mu.RUnlock()

	return s.fetchAndCache(ctx)
}

func (s *Service) fetchAndCache(ctx context.Context) (*PriceData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cache != nil && time.Now().Before(s.cacheExpiry) {
		return s.cache, nil
	}

	url := fmt.Sprintf(
		"https://api.coingecko.com/api/v3/simple/price?ids=%s&vs_currencies=%s&include_24hr_change=true&include_market_cap=true&include_24hr_vol=true",
		s.coinID,
		s.currency,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		if s.cache != nil {
			return s.cache, nil
		}
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		if s.cache != nil {
			return s.cache, nil
		}
		return nil, fmt.Errorf("failed to fetch price: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if s.cache != nil {
			return s.cache, nil
		}
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result coingeckoResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		if s.cache != nil {
			return s.cache, nil
		}
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	coinData, ok := result[s.coinID]
	if !ok {
		if s.cache != nil {
			return s.cache, nil
		}
		return nil, fmt.Errorf("coin %s not found in response", s.coinID)
	}

	s.cache = &PriceData{
		Price:       coinData.USD,
		Currency:    s.currency,
		Change24h:   coinData.USDChange24h,
		MarketCap:   coinData.USDMarketCap,
		Volume24h:   coinData.USDVolume24h,
		LastUpdated: time.Now(),
	}
	s.cacheExpiry = time.Now().Add(s.cacheTTL)

	if s.eventBus != nil {
		s.eventBus.PublishPriceUpdate(s.cache)
	}

	return s.cache, nil
}

func (s *Service) StartBackgroundRefresh(ctx context.Context) {
	go func() {
		s.fetchAndCache(ctx)

		ticker := time.NewTicker(s.cacheTTL)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.fetchAndCache(ctx)
			}
		}
	}()
}
