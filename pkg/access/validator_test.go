package access

import (
	"testing"

	"github.com/google/go-github/v74/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewValidator(t *testing.T) {
	client := github.NewClient(nil)
	userEmail := "test@example.com"

	validator := NewValidator(client, userEmail)

	assert.NotNil(t, validator)
	assert.Equal(t, userEmail, validator.userEmail)
	assert.Equal(t, client, validator.client)
	assert.False(t, validator.initialized)
	assert.NotNil(t, validator.accessibleRepos)
}

func TestValidator_Initialize(t *testing.T) {
	client := github.NewClient(nil)
	validator := NewValidator(client, "test@example.com")

	// First initialization should succeed
	err := validator.Initialize()
	require.NoError(t, err)
	assert.True(t, validator.initialized)
	assert.NotEmpty(t, validator.accessibleRepos)

	// Second initialization should be a no-op
	oldRepos := validator.accessibleRepos
	err = validator.Initialize()
	require.NoError(t, err)
	assert.Equal(t, oldRepos, validator.accessibleRepos)
}

func TestValidator_IsRepositoryAccessible(t *testing.T) {
	client := github.NewClient(nil)
	validator := NewValidator(client, "test@example.com")

	// Should fail when not initialized
	accessible, err := validator.IsRepositoryAccessible("github.com/user/repo1")
	assert.Error(t, err)
	assert.False(t, accessible)
	assert.Contains(t, err.Error(), "validator not initialized")

	// Initialize the validator
	err = validator.Initialize()
	require.NoError(t, err)

	// Test cases for accessible repositories (from dummy data)
	testCases := []struct {
		name        string
		repoURL     string
		expectAccess bool
		expectError  bool
	}{
		{
			name:        "accessible repo - exact match",
			repoURL:     "github.com/user/repo1",
			expectAccess: true,
			expectError:  false,
		},
		{
			name:        "accessible repo - https URL",
			repoURL:     "https://github.com/user/repo1",
			expectAccess: true,
			expectError:  false,
		},
		{
			name:        "accessible repo - short format",
			repoURL:     "user/repo1",
			expectAccess: true,
			expectError:  false,
		},
		{
			name:        "accessible repo - with .git suffix",
			repoURL:     "https://github.com/user/repo1.git",
			expectAccess: true,
			expectError:  false,
		},
		{
			name:        "inaccessible repo",
			repoURL:     "github.com/other/repo",
			expectAccess: false,
			expectError:  false,
		},
		{
			name:        "empty URL",
			repoURL:     "",
			expectAccess: false,
			expectError:  true,
		},
		{
			name:        "invalid URL format",
			repoURL:     "invalid-url",
			expectAccess: false,
			expectError:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			accessible, err := validator.IsRepositoryAccessible(tc.repoURL)
			
			if tc.expectError {
				assert.Error(t, err)
				assert.False(t, accessible)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectAccess, accessible)
			}
		})
	}
}

func TestValidator_GetAccessibleRepositories(t *testing.T) {
	client := github.NewClient(nil)
	validator := NewValidator(client, "test@example.com")

	// Before initialization
	repos := validator.GetAccessibleRepositories()
	assert.Empty(t, repos)

	// After initialization
	err := validator.Initialize()
	require.NoError(t, err)

	repos = validator.GetAccessibleRepositories()
	assert.NotEmpty(t, repos)
	
	// Should contain the dummy repositories
	expectedRepos := []string{
		"github.com/user/repo1",
		"github.com/user/repo2",
		"github.com/org/public-repo",
		"github.com/github/github-mcp-server",
	}
	
	assert.Len(t, repos, len(expectedRepos))
	for _, expectedRepo := range expectedRepos {
		assert.Contains(t, repos, expectedRepo)
	}
}

func TestNormalizeRepositoryURL(t *testing.T) {
	testCases := []struct {
		name        string
		input       string
		expected    string
		expectError bool
	}{
		{
			name:        "https URL",
			input:       "https://github.com/owner/repo",
			expected:    "github.com/owner/repo",
			expectError: false,
		},
		{
			name:        "https URL with .git suffix",
			input:       "https://github.com/owner/repo.git",
			expected:    "github.com/owner/repo",
			expectError: false,
		},
		{
			name:        "http URL",
			input:       "http://github.com/owner/repo",
			expected:    "github.com/owner/repo",
			expectError: false,
		},
		{
			name:        "github.com prefix",
			input:       "github.com/owner/repo",
			expected:    "github.com/owner/repo",
			expectError: false,
		},
		{
			name:        "github.com prefix with .git",
			input:       "github.com/owner/repo.git",
			expected:    "github.com/owner/repo",
			expectError: false,
		},
		{
			name:        "short format",
			input:       "owner/repo",
			expected:    "github.com/owner/repo",
			expectError: false,
		},
		{
			name:        "short format with .git",
			input:       "owner/repo.git",
			expected:    "github.com/owner/repo",
			expectError: false,
		},
		{
			name:        "empty string",
			input:       "",
			expected:    "",
			expectError: true,
		},
		{
			name:        "invalid format - just owner",
			input:       "owner",
			expected:    "",
			expectError: true,
		},
		{
			name:        "invalid format - too many slashes",
			input:       "github.com/owner/repo/extra",
			expected:    "",
			expectError: true,
		},
		{
			name:        "invalid URL",
			input:       "https://invalid-url",
			expected:    "",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := normalizeRepositoryURL(tc.input)
			
			if tc.expectError {
				assert.Error(t, err)
				assert.Empty(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestValidator_ConcurrentAccess(t *testing.T) {
	client := github.NewClient(nil)
	validator := NewValidator(client, "test@example.com")

	// Initialize the validator
	err := validator.Initialize()
	require.NoError(t, err)

	// Test concurrent access to IsRepositoryAccessible
	const numGoroutines = 10
	const numCalls = 100

	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer func() { done <- true }()
			
			for j := 0; j < numCalls; j++ {
				accessible, err := validator.IsRepositoryAccessible("github.com/user/repo1")
				assert.NoError(t, err)
				assert.True(t, accessible)
				
				repos := validator.GetAccessibleRepositories()
				assert.NotEmpty(t, repos)
			}
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}
