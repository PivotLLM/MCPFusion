/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package knowledge_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/PivotLLM/MCPFusion/db"
	"github.com/PivotLLM/MCPFusion/global"
	"github.com/PivotLLM/MCPFusion/providers/knowledge"
)

// mockDB is a minimal in-memory implementation of db.Database for testing.
type mockDB struct {
	store map[string]*db.KnowledgeEntry
}

func newMockDB() *mockDB {
	return &mockDB{store: make(map[string]*db.KnowledgeEntry)}
}

func (m *mockDB) key(userID, domain, key string) string {
	return fmt.Sprintf("%s::%s::%s", userID, domain, key)
}

func (m *mockDB) SetKnowledge(userID string, entry *db.KnowledgeEntry) error {
	m.store[m.key(userID, entry.Domain, entry.Key)] = entry
	return nil
}
func (m *mockDB) GetKnowledge(userID, domain, key string) (*db.KnowledgeEntry, error) {
	e, ok := m.store[m.key(userID, domain, key)]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return e, nil
}
func (m *mockDB) ListKnowledge(userID, domain string) ([]db.KnowledgeEntry, error) {
	return nil, nil
}
func (m *mockDB) DeleteKnowledge(userID, domain, key string) error {
	delete(m.store, m.key(userID, domain, key))
	return nil
}
func (m *mockDB) RenameKnowledge(userID, domain, oldKey, newKey string) error {
	k := m.key(userID, domain, oldKey)
	e, ok := m.store[k]
	if !ok {
		return fmt.Errorf("not found")
	}
	delete(m.store, k)
	e.Key = newKey
	m.store[m.key(userID, domain, newKey)] = e
	return nil
}
func (m *mockDB) SearchKnowledge(userID, query string) ([]db.KnowledgeEntry, error) {
	return nil, nil
}

// Stub out the remainder of the db.Database interface.
func (m *mockDB) AddAPIToken(_ string) (string, string, error)    { return "", "", nil }
func (m *mockDB) ValidateAPIToken(_ string) (bool, string, error) { return false, "", nil }
func (m *mockDB) DeleteAPIToken(_ string) error                   { return nil }
func (m *mockDB) ListAPITokens() ([]db.APITokenMetadata, error)   { return nil, nil }
func (m *mockDB) GetAPITokenMetadata(_ string) (*db.APITokenMetadata, error) {
	return nil, nil
}
func (m *mockDB) ResolveAPIToken(_ string) (string, error) { return "", nil }
func (m *mockDB) StoreOAuthToken(_, _ string, _ *db.OAuthTokenData) error {
	return nil
}
func (m *mockDB) GetOAuthToken(_, _ string) (*db.OAuthTokenData, error) { return nil, nil }
func (m *mockDB) DeleteOAuthToken(_, _ string) error                    { return nil }
func (m *mockDB) ListOAuthTokens(_ string) (map[string]*db.OAuthTokenData, error) {
	return nil, nil
}
func (m *mockDB) StoreCredentials(_, _ string, _ *db.ServiceCredentials) error {
	return nil
}
func (m *mockDB) GetCredentials(_, _ string) (*db.ServiceCredentials, error) { return nil, nil }
func (m *mockDB) DeleteCredentials(_, _ string) error                        { return nil }
func (m *mockDB) ListCredentials(_ string) (map[string]*db.ServiceCredentials, error) {
	return nil, nil
}
func (m *mockDB) CreateAuthCode(_, _ string, _ time.Duration) (string, error) { return "", nil }
func (m *mockDB) ValidateAuthCode(_ string) (string, string, error)           { return "", "", nil }
func (m *mockDB) CleanupExpiredAuthCodes() error                              { return nil }
func (m *mockDB) GetTenantInfo(_ string) (*db.TenantInfo, error)              { return nil, nil }
func (m *mockDB) ListTenants() ([]db.TenantInfo, error)                       { return nil, nil }
func (m *mockDB) CreateUser(_ string) (*db.UserMetadata, error)               { return nil, nil }
func (m *mockDB) GetUser(_ string) (*db.UserMetadata, error)                  { return nil, nil }
func (m *mockDB) ListUsers() ([]db.UserMetadata, error)                       { return nil, nil }
func (m *mockDB) DeleteUser(_ string) error                                   { return nil }
func (m *mockDB) LinkAPIKey(_, _ string) error                                { return nil }
func (m *mockDB) UnlinkAPIKey(_ string) error                                 { return nil }
func (m *mockDB) GetUserByAPIKey(_ string) (string, error)                    { return "", nil }
func (m *mockDB) AutoMigrateKeys() error                                      { return nil }
func (m *mockDB) Close() error                                                { return nil }
func (m *mockDB) Backup(_ string) error                                       { return nil }

// Ensure mockDB satisfies the interface at compile time.
var _ db.Database = (*mockDB)(nil)

// --- Tests ---

func TestRegisterTools_NilDB_ReturnsNil(t *testing.T) {
	p := knowledge.New()
	tools := p.RegisterTools()
	require.Nil(t, tools, "expected nil when no database is configured")
}

func TestRegisterTools_WithDB_ReturnsFiveTools(t *testing.T) {
	p := knowledge.New(
		knowledge.WithDatabase(newMockDB()),
	)
	tools := p.RegisterTools()
	require.Len(t, tools, 5)

	names := make([]string, len(tools))
	for i, t := range tools {
		names[i] = t.Name
	}
	require.Contains(t, names, "knowledge_set")
	require.Contains(t, names, "knowledge_get")
	require.Contains(t, names, "knowledge_delete")
	require.Contains(t, names, "knowledge_rename")
	require.Contains(t, names, "knowledge_search")
}

func TestHandleKnowledgeSet_NoTenantContext_ReturnsError(t *testing.T) {
	p := knowledge.New(
		knowledge.WithDatabase(newMockDB()),
		knowledge.WithUserIDExtractor(func(ctx context.Context) (string, error) {
			return "", fmt.Errorf("no tenant context available")
		}),
	)
	tools := p.RegisterTools()
	require.NotNil(t, tools)

	var setTool *global.ToolDefinition
	for i := range tools {
		if tools[i].Name == "knowledge_set" {
			setTool = &tools[i]
			break
		}
	}
	require.NotNil(t, setTool)

	// Call without an MCP context — extractor will return an error.
	_, err := setTool.Handler(map[string]interface{}{
		"domain":  "test",
		"key":     "k",
		"content": "v",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no tenant context available")
}
