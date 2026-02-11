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

// PriceData represents the current price information
type PriceData struct {
	Price          float64   `json:"price"`
	Currency       string    `json:"currency"`
	Change24h      float64   `json:"change24h"`
	MarketCap      float64   `json:"marketCap,omitempty"`
	Volume24h      float64   `json:"volume24h,omitempty"`
	LastUpdated    time.Time `json:"lastUpdated"`
}

// Service fetches and caches cryptocurrency prices
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

// SetEventBus sets the event bus for publishing price update events
func (s *Service) SetEventBus(bus *events.Bus) {
	s.eventBus = bus
}

// CoinGecko API response structure
type coingeckoResponse map[string]struct {
	USD          float64 `json:"usd"`
	USDChange24h float64 `json:"usd_24h_change"`
	USDMarketCap float64 `json:"usd_market_cap"`
	USDVolume24h float64 `json:"usd_24h_vol"`
}

// NewService creates a new price service
func NewService(coinID, currency string, cacheTTL time.Duration) *Service {
	return &Service{
		cacheTTL:   cacheTTL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		coinID:     coinID,
		currency:   currency,
	}
}

// GetPrice returns the current price, fetching from API if cache is expired
func (s *Service) GetPrice(ctx context.Context) (*PriceData, error) {
	s.mu.RLock()
	if s.cache != nil && time.Now().Before(s.cacheExpiry) {
		defer s.mu.RUnlock()
		return s.cache, nil
	}
	s.mu.RUnlock()

	// Cache expired or empty, fetch new price
	return s.fetchAndCache(ctx)
}

// fetchAndCache fetches price from CoinGecko and updates cache
func (s *Service) fetchAndCache(ctx context.Context) (*PriceData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check cache (another goroutine might have updated it)
	if s.cache != nil && time.Now().Before(s.cacheExpiry) {
		return s.cache, nil
	}

	// Fetch from CoinGecko
	url := fmt.Sprintf(
		"https://api.coingecko.com/api/v3/simple/price?ids=%s&vs_currencies=%s&include_24hr_change=true&include_market_cap=true&include_24hr_vol=true",
		s.coinID,
		s.currency,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		// Return stale cache if available
		if s.cache != nil {
			return s.cache, nil
		}
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		// Return stale cache if available
		if s.cache != nil {
			return s.cache, nil
		}
		return nil, fmt.Errorf("failed to fetch price: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Return stale cache if available
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

	// Publish price update event
	if s.eventBus != nil {
		s.eventBus.PublishPriceUpdate(s.cache)
	}

	return s.cache, nil
}

// StartBackgroundRefresh starts a background goroutine to keep the cache fresh
func (s *Service) StartBackgroundRefresh(ctx context.Context) {
	go func() {
		// Initial fetch
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
