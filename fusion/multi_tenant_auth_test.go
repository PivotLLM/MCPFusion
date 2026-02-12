/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package fusion

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// mockCache implements the Cache interface for testing
type mockCache struct {
	mu    sync.RWMutex
	items map[string]interface{}
}

func newMockCache() *mockCache {
	return &mockCache{
		items: make(map[string]interface{}),
	}
}

func (c *mockCache) Get(key string) (interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	val, ok := c.items[key]
	if !ok {
		return nil, fmt.Errorf("key not found: %s", key)
	}
	return val, nil
}

func (c *mockCache) Set(key string, value interface{}, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = value
	return nil
}

func (c *mockCache) Delete(key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)
	return nil
}

func (c *mockCache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]interface{})
	return nil
}

func (c *mockCache) Has(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	_, ok := c.items[key]
	return ok
}

// mockStrategy implements the AuthStrategy interface for testing
type mockStrategy struct {
	authType        AuthType
	supportsRefresh bool
	refreshFunc     func(ctx context.Context, tokenInfo *TokenInfo, config map[string]interface{}) (*TokenInfo, error)
}

func (s *mockStrategy) Authenticate(_ context.Context, _ map[string]interface{}) (*TokenInfo, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *mockStrategy) RefreshToken(ctx context.Context, tokenInfo *TokenInfo, config map[string]interface{}) (*TokenInfo, error) {
	if s.refreshFunc != nil {
		return s.refreshFunc(ctx, tokenInfo, config)
	}
	return nil, fmt.Errorf("refresh not implemented")
}

func (s *mockStrategy) GetAuthType() AuthType {
	return s.authType
}

func (s *mockStrategy) SupportsRefresh() bool {
	return s.supportsRefresh
}

func (s *mockStrategy) ApplyAuth(req *http.Request, tokenInfo *TokenInfo) error {
	if tokenInfo == nil {
		return fmt.Errorf("token info is nil")
	}
	req.Header.Set("Authorization", tokenInfo.GetAuthorizationHeader())
	return nil
}

func TestRefreshIfPossible_NilTenantContext(t *testing.T) {
	cache := newMockCache()
	manager := NewMultiTenantAuthManager(nil, cache, nil)

	authConfig := AuthConfig{
		Type:   AuthTypeOAuth2External,
		Config: map[string]interface{}{},
	}

	_, err := manager.RefreshIfPossible(context.Background(), nil, authConfig)
	if err == nil {
		t.Fatal("expected error for nil tenant context, got nil")
	}

	if !strings.Contains(err.Error(), "tenant context is required") {
		t.Errorf("expected error to contain 'tenant context is required', got: %s", err.Error())
	}
}

func TestRefreshIfPossible_NoToken(t *testing.T) {
	cache := newMockCache()
	manager := NewMultiTenantAuthManager(nil, cache, nil)

	tenantCtx := &TenantContext{
		TenantHash:  "abc123def456",
		ServiceName: "test_service",
		CreatedAt:   time.Now(),
	}

	authConfig := AuthConfig{
		Type:   AuthTypeOAuth2External,
		Config: map[string]interface{}{},
	}

	_, err := manager.RefreshIfPossible(context.Background(), tenantCtx, authConfig)
	if err == nil {
		t.Fatal("expected error when no token is cached, got nil")
	}

	if !strings.Contains(err.Error(), "no token found") {
		t.Errorf("expected error to contain 'no token found', got: %s", err.Error())
	}
}

func TestRefreshIfPossible_UnsupportedStrategy(t *testing.T) {
	cache := newMockCache()
	manager := NewMultiTenantAuthManager(nil, cache, nil)

	tenantCtx := &TenantContext{
		TenantHash:  "abc123def456",
		ServiceName: "test_service",
		CreatedAt:   time.Now(),
	}

	// Store a token in the cache so getCachedToken finds it
	cacheKey := fmt.Sprintf("tenant:%s:token:%s", tenantCtx.TenantHash, tenantCtx.ServiceName)
	cache.Set(cacheKey, &TokenInfo{
		AccessToken:  "some_token",
		RefreshToken: "some_refresh",
	}, time.Hour)

	// Use an auth type with no registered strategy
	authConfig := AuthConfig{
		Type:   AuthType("nonexistent_auth_type"),
		Config: map[string]interface{}{},
	}

	_, err := manager.RefreshIfPossible(context.Background(), tenantCtx, authConfig)
	if err == nil {
		t.Fatal("expected error for unsupported strategy, got nil")
	}

	if !strings.Contains(err.Error(), "unsupported authentication type") {
		t.Errorf("expected error to contain 'unsupported authentication type', got: %s", err.Error())
	}
}

func TestRefreshIfPossible_StrategyDoesNotSupportRefresh(t *testing.T) {
	cache := newMockCache()
	manager := NewMultiTenantAuthManager(nil, cache, nil)

	// Register a strategy that does not support refresh
	strategy := &mockStrategy{
		authType:        AuthTypeOAuth2External,
		supportsRefresh: false,
	}
	manager.RegisterStrategy(strategy)

	tenantCtx := &TenantContext{
		TenantHash:  "abc123def456",
		ServiceName: "test_service",
		CreatedAt:   time.Now(),
	}

	cacheKey := fmt.Sprintf("tenant:%s:token:%s", tenantCtx.TenantHash, tenantCtx.ServiceName)
	cache.Set(cacheKey, &TokenInfo{
		AccessToken:  "some_token",
		RefreshToken: "some_refresh",
	}, time.Hour)

	authConfig := AuthConfig{
		Type:   AuthTypeOAuth2External,
		Config: map[string]interface{}{},
	}

	_, err := manager.RefreshIfPossible(context.Background(), tenantCtx, authConfig)
	if err == nil {
		t.Fatal("expected error when strategy does not support refresh, got nil")
	}

	if !strings.Contains(err.Error(), "token refresh not supported") {
		t.Errorf("expected error to contain 'token refresh not supported', got: %s", err.Error())
	}
}

func TestRefreshIfPossible_NoRefreshToken(t *testing.T) {
	cache := newMockCache()
	manager := NewMultiTenantAuthManager(nil, cache, nil)

	strategy := &mockStrategy{
		authType:        AuthTypeOAuth2External,
		supportsRefresh: true,
	}
	manager.RegisterStrategy(strategy)

	tenantCtx := &TenantContext{
		TenantHash:  "abc123def456",
		ServiceName: "test_service",
		CreatedAt:   time.Now(),
	}

	// Store a token without a refresh token
	cacheKey := fmt.Sprintf("tenant:%s:token:%s", tenantCtx.TenantHash, tenantCtx.ServiceName)
	cache.Set(cacheKey, &TokenInfo{
		AccessToken:  "some_token",
		RefreshToken: "", // no refresh token
	}, time.Hour)

	authConfig := AuthConfig{
		Type:   AuthTypeOAuth2External,
		Config: map[string]interface{}{},
	}

	_, err := manager.RefreshIfPossible(context.Background(), tenantCtx, authConfig)
	if err == nil {
		t.Fatal("expected error when token has no refresh token, got nil")
	}

	if !strings.Contains(err.Error(), "no refresh token available") {
		t.Errorf("expected error to contain 'no refresh token available', got: %s", err.Error())
	}
}

func TestRefreshIfPossible_RefreshFails(t *testing.T) {
	cache := newMockCache()
	manager := NewMultiTenantAuthManager(nil, cache, nil)

	refreshErr := fmt.Errorf("upstream token endpoint unavailable")
	strategy := &mockStrategy{
		authType:        AuthTypeOAuth2External,
		supportsRefresh: true,
		refreshFunc: func(_ context.Context, _ *TokenInfo, _ map[string]interface{}) (*TokenInfo, error) {
			return nil, refreshErr
		},
	}
	manager.RegisterStrategy(strategy)

	tenantCtx := &TenantContext{
		TenantHash:  "abc123def456",
		ServiceName: "test_service",
		CreatedAt:   time.Now(),
	}

	cacheKey := fmt.Sprintf("tenant:%s:token:%s", tenantCtx.TenantHash, tenantCtx.ServiceName)
	cache.Set(cacheKey, &TokenInfo{
		AccessToken:  "old_access_token",
		RefreshToken: "valid_refresh_token",
	}, time.Hour)

	authConfig := AuthConfig{
		Type:   AuthTypeOAuth2External,
		Config: map[string]interface{}{},
	}

	_, err := manager.RefreshIfPossible(context.Background(), tenantCtx, authConfig)
	if err == nil {
		t.Fatal("expected error when refresh fails, got nil")
	}

	// The error from the strategy should be propagated directly (not wrapped)
	if err.Error() != refreshErr.Error() {
		t.Errorf("expected error '%s', got: '%s'", refreshErr.Error(), err.Error())
	}
}

func TestRefreshIfPossible_Success(t *testing.T) {
	cache := newMockCache()
	manager := NewMultiTenantAuthManager(nil, cache, nil)

	newExpiry := time.Now().Add(1 * time.Hour)
	refreshedToken := &TokenInfo{
		AccessToken:  "new_access_token",
		RefreshToken: "new_refresh_token",
		TokenType:    "Bearer",
		ExpiresAt:    &newExpiry,
		Scope:        []string{"openid", "email"},
		Metadata:     map[string]string{"source": "refresh"},
	}

	strategy := &mockStrategy{
		authType:        AuthTypeOAuth2External,
		supportsRefresh: true,
		refreshFunc: func(_ context.Context, _ *TokenInfo, _ map[string]interface{}) (*TokenInfo, error) {
			return refreshedToken, nil
		},
	}
	manager.RegisterStrategy(strategy)

	tenantCtx := &TenantContext{
		TenantHash:  "abc123def456",
		ServiceName: "test_service",
		CreatedAt:   time.Now(),
	}

	cacheKey := fmt.Sprintf("tenant:%s:token:%s", tenantCtx.TenantHash, tenantCtx.ServiceName)
	cache.Set(cacheKey, &TokenInfo{
		AccessToken:  "old_access_token",
		RefreshToken: "old_refresh_token",
		TokenType:    "Bearer",
	}, time.Hour)

	authConfig := AuthConfig{
		Type:   AuthTypeOAuth2External,
		Config: map[string]interface{}{"clientId": "test-client"},
	}

	result, err := manager.RefreshIfPossible(context.Background(), tenantCtx, authConfig)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil token result")
	}

	if result.AccessToken != "new_access_token" {
		t.Errorf("AccessToken = %s, want new_access_token", result.AccessToken)
	}

	if result.RefreshToken != "new_refresh_token" {
		t.Errorf("RefreshToken = %s, want new_refresh_token", result.RefreshToken)
	}

	if result.TokenType != "Bearer" {
		t.Errorf("TokenType = %s, want Bearer", result.TokenType)
	}

	if result.ExpiresAt == nil {
		t.Error("ExpiresAt should not be nil")
	}

	// Verify the refreshed token was cached
	cachedVal, cacheErr := cache.Get(cacheKey)
	if cacheErr != nil {
		t.Fatalf("expected refreshed token to be cached, got error: %v", cacheErr)
	}

	cachedToken, ok := cachedVal.(*TokenInfo)
	if !ok {
		t.Fatal("cached value is not a *TokenInfo")
	}

	if cachedToken.AccessToken != "new_access_token" {
		t.Errorf("cached AccessToken = %s, want new_access_token", cachedToken.AccessToken)
	}
}

// TestInvalidateToken_ConcurrentSameKey verifies that calling InvalidateToken
// concurrently for the same tenant+service combination does not race or panic.
// With nil database and nil cache, no I/O occurs but the per-key mutex logic
// in invalidationLocks (sync.Map with *sync.Mutex values) is fully exercised.
func TestInvalidateToken_ConcurrentSameKey(t *testing.T) {
	manager := NewMultiTenantAuthManager(nil, nil, nil)

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			tc := &TenantContext{
				TenantHash:  "shared_tenant_hash",
				ServiceName: "shared_service",
				CreatedAt:   time.Now(),
			}
			manager.InvalidateToken(tc)
		}()
	}

	wg.Wait()
}

// TestInvalidateToken_ConcurrentDifferentKeys verifies that calling InvalidateToken
// concurrently for different tenant+service combinations does not race or panic.
// Each goroutine targets a unique key, exercising concurrent LoadOrStore calls
// on the invalidationLocks sync.Map.
func TestInvalidateToken_ConcurrentDifferentKeys(t *testing.T) {
	manager := NewMultiTenantAuthManager(nil, nil, nil)

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			tc := &TenantContext{
				TenantHash:  fmt.Sprintf("tenant_%d", idx),
				ServiceName: fmt.Sprintf("service_%d", idx),
				CreatedAt:   time.Now(),
			}
			manager.InvalidateToken(tc)
		}(i)
	}

	wg.Wait()
}

// TestInvalidateToken_ConcurrentMixed verifies concurrent InvalidateToken calls
// with a mix of shared and unique tenant+service keys. This exercises both the
// LoadOrStore contention path (same key) and the concurrent creation path
// (different keys) simultaneously.
func TestInvalidateToken_ConcurrentMixed(t *testing.T) {
	manager := NewMultiTenantAuthManager(nil, nil, nil)

	const goroutinesPerKey = 20
	const uniqueKeys = 10
	total := goroutinesPerKey * uniqueKeys
	var wg sync.WaitGroup
	wg.Add(total)

	for k := 0; k < uniqueKeys; k++ {
		for g := 0; g < goroutinesPerKey; g++ {
			go func(keyIdx int) {
				defer wg.Done()
				tc := &TenantContext{
					TenantHash:  fmt.Sprintf("tenant_%d", keyIdx),
					ServiceName: fmt.Sprintf("service_%d", keyIdx),
					CreatedAt:   time.Now(),
				}
				manager.InvalidateToken(tc)
			}(k)
		}
	}

	wg.Wait()
}

// TestInvalidateToken_NilTenantContext verifies that InvalidateToken returns
// immediately without panicking when given a nil TenantContext.
func TestInvalidateToken_NilTenantContext(t *testing.T) {
	manager := NewMultiTenantAuthManager(nil, nil, nil)
	// Must not panic
	manager.InvalidateToken(nil)
}
