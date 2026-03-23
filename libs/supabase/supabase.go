// Package supabase provides Supabase-specific convenience operations
// including Row Level Security, auth/JWT helpers, Edge Functions,
// and storage operations.
package supabase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	gopgbase "github.com/goozt/gopgbase"
)

// SupabaseLibrary provides Supabase-specific operations.
type SupabaseLibrary struct {
	client     *gopgbase.Client
	projectURL string
	apiKey     string
	serviceKey string
}

// Config holds Supabase API configuration for non-database operations.
type Config struct {
	// ProjectURL is the Supabase project URL (e.g., https://xxx.supabase.co).
	ProjectURL string `json:"project_url"`
	// APIKey is the anon key for public operations.
	APIKey string `json:"api_key"`
	// ServiceRoleKey is the service role key for admin operations.
	ServiceRoleKey string `json:"service_role_key"`
}

// NewSupabaseLibrary creates a new SupabaseLibrary backed by the given Client.
func NewSupabaseLibrary(client *gopgbase.Client, cfg ...Config) (*SupabaseLibrary, error) {
	if client == nil {
		return nil, fmt.Errorf("gopgbase/supabase: client must not be nil")
	}
	lib := &SupabaseLibrary{client: client}
	if len(cfg) > 0 {
		lib.projectURL = cfg[0].ProjectURL
		lib.apiKey = cfg[0].APIKey
		lib.serviceKey = cfg[0].ServiceRoleKey
	}
	return lib, nil
}

// AuthLibrary provides JWT validation and user extraction.
type AuthLibrary struct {
	client     *gopgbase.Client
	projectURL string
	apiKey     string
}

// Auth returns an AuthLibrary for JWT-based authentication operations.
func (l *SupabaseLibrary) Auth(_ context.Context) *AuthLibrary {
	return &AuthLibrary{
		client:     l.client,
		projectURL: l.projectURL,
		apiKey:     l.apiKey,
	}
}

// ValidateJWT validates a Supabase JWT token by checking with the auth server.
func (a *AuthLibrary) ValidateJWT(ctx context.Context, token string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		a.projectURL+"/auth/v1/user", nil)
	if err != nil {
		return nil, fmt.Errorf("gopgbase/supabase: validate jwt: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("apikey", a.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gopgbase/supabase: validate jwt: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gopgbase/supabase: validate jwt: HTTP %d", resp.StatusCode)
	}

	var user map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("gopgbase/supabase: validate jwt decode: %w", err)
	}
	return user, nil
}

// GetUserFromJWT extracts the user ID from a Supabase JWT by querying
// the auth.users table via the database connection.
func (a *AuthLibrary) GetUserFromJWT(ctx context.Context, token string) (string, error) {
	user, err := a.ValidateJWT(ctx, token)
	if err != nil {
		return "", err
	}
	id, ok := user["id"].(string)
	if !ok {
		return "", fmt.Errorf("gopgbase/supabase: user id not found in JWT response")
	}
	return id, nil
}

// EnableRLS enables Row Level Security on a table and creates a policy.
//
// Parameters:
//   - table: the table to enable RLS on
//   - policyName: name of the RLS policy
//   - policySQL: the policy definition (e.g., "USING (auth.uid() = user_id)")
func (l *SupabaseLibrary) EnableRLS(ctx context.Context, table, policyName, policySQL string) error {
	enableQuery := fmt.Sprintf("ALTER TABLE %s ENABLE ROW LEVEL SECURITY", quoteIdentifier(table))
	if _, err := l.client.DataStore().ExecContext(ctx, enableQuery); err != nil {
		return fmt.Errorf("gopgbase/supabase: enable rls: %w", err)
	}

	policyQuery := fmt.Sprintf(
		"CREATE POLICY %s ON %s %s",
		quoteIdentifier(policyName), quoteIdentifier(table), policySQL,
	)
	if _, err := l.client.DataStore().ExecContext(ctx, policyQuery); err != nil {
		return fmt.Errorf("gopgbase/supabase: create policy: %w", err)
	}

	return nil
}

// InvokeEdgeFunction calls a Supabase Edge Function with the given payload.
func (l *SupabaseLibrary) InvokeEdgeFunction(ctx context.Context, functionName string, payload any) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("gopgbase/supabase: marshal payload: %w", err)
	}

	url := fmt.Sprintf("%s/functions/v1/%s", l.projectURL, functionName)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("gopgbase/supabase: invoke edge function: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+l.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gopgbase/supabase: invoke edge function: %w", err)
	}
	defer resp.Body.Close()

	result, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gopgbase/supabase: read edge function response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("gopgbase/supabase: edge function %s returned HTTP %d: %s",
			functionName, resp.StatusCode, string(result))
	}

	return result, nil
}

// UploadFile uploads a file to Supabase Storage.
func (l *SupabaseLibrary) UploadFile(ctx context.Context, bucket, path string, data []byte) error {
	url := fmt.Sprintf("%s/storage/v1/object/%s/%s", l.projectURL, bucket, path)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("gopgbase/supabase: upload file: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+l.serviceKey)
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("gopgbase/supabase: upload file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("gopgbase/supabase: upload file failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// GenerateSignedURL creates a signed URL for temporary access to a storage object.
func (l *SupabaseLibrary) GenerateSignedURL(ctx context.Context, bucket, path string, expires time.Duration) (string, error) {
	payload := map[string]any{
		"expiresIn": int(expires.Seconds()),
	}
	body, _ := json.Marshal(payload)

	url := fmt.Sprintf("%s/storage/v1/object/sign/%s/%s", l.projectURL, bucket, path)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("gopgbase/supabase: generate signed url: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+l.serviceKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("gopgbase/supabase: generate signed url: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("gopgbase/supabase: decode signed url response: %w", err)
	}

	signedURL, ok := result["signedURL"].(string)
	if !ok {
		return "", fmt.Errorf("gopgbase/supabase: signed url not found in response")
	}

	return l.projectURL + "/storage/v1" + signedURL, nil
}

// UserManager provides user management operations.
type UserManager struct {
	client     *gopgbase.Client
	projectURL string
	serviceKey string
}

// UserManager returns a UserManager for admin user operations.
func (l *SupabaseLibrary) UserManager(_ context.Context) *UserManager {
	return &UserManager{
		client:     l.client,
		projectURL: l.projectURL,
		serviceKey: l.serviceKey,
	}
}

// CreateUser creates a new Supabase auth user via the admin API.
func (um *UserManager) CreateUser(ctx context.Context, email, password string, metadata map[string]any) (map[string]any, error) {
	payload := map[string]any{
		"email":    email,
		"password": password,
	}
	if metadata != nil {
		payload["user_metadata"] = metadata
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		um.projectURL+"/auth/v1/admin/users", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("gopgbase/supabase: create user: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+um.serviceKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", um.serviceKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gopgbase/supabase: create user: %w", err)
	}
	defer resp.Body.Close()

	var user map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("gopgbase/supabase: create user decode: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("gopgbase/supabase: create user failed (HTTP %d): %v", resp.StatusCode, user)
	}

	return user, nil
}

// ListUsers lists all auth users via the admin API.
func (um *UserManager) ListUsers(ctx context.Context) ([]map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		um.projectURL+"/auth/v1/admin/users", nil)
	if err != nil {
		return nil, fmt.Errorf("gopgbase/supabase: list users: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+um.serviceKey)
	req.Header.Set("apikey", um.serviceKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gopgbase/supabase: list users: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Users []map[string]any `json:"users"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("gopgbase/supabase: list users decode: %w", err)
	}

	return result.Users, nil
}

func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
