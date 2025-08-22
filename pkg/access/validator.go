package access

import (
	"fmt"
	"net/url"
	"strings"
	"sync"
)

// Validator handles repository access validation for a specific user
type Validator struct {
	userEmail       string
	accessibleRepos map[string]struct{} // Set implementation for efficient lookups
	mu              sync.RWMutex
	initialized     bool
}

// NewValidator creates a new access validator instance
func NewValidator(userEmail string) *Validator {
	return &Validator{
		userEmail:       userEmail,
		accessibleRepos: make(map[string]struct{}),
	}
}

// Initialize fetches and caches the list of accessible repositories for the user
// This is a blocking operation that must complete before the server can start
func (v *Validator) Initialize() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.initialized {
		return nil
	}

	repos, err := v.fetchAccessibleRepositories()
	if err != nil {
		return fmt.Errorf("failed to fetch accessible repositories: %w", err)
	}

	// Clear any existing data and populate with fresh results
	v.accessibleRepos = make(map[string]struct{})
	for _, repo := range repos {
		v.accessibleRepos[repo] = struct{}{}
	}

	v.initialized = true
	return nil
}

// IsRepositoryAccessible checks if the given repository URL is accessible to the user
// Returns true if the repository is in the cached accessible list
func (v *Validator) IsRepositoryAccessible(repoURL string) (bool, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if !v.initialized {
		return false, fmt.Errorf("validator not initialized")
	}

	normalizedURL, err := normalizeRepositoryURL(repoURL)
	if err != nil {
		return false, fmt.Errorf("failed to normalize repository URL: %w", err)
	}

	_, exists := v.accessibleRepos[normalizedURL]
	return exists, nil
}

// GetAccessibleRepositories returns a copy of the accessible repositories set
// This is useful for debugging or administrative purposes
func (v *Validator) GetAccessibleRepositories() []string {
	v.mu.RLock()
	defer v.mu.RUnlock()

	repos := make([]string, 0, len(v.accessibleRepos))
	for repo := range v.accessibleRepos {
		repos = append(repos, repo)
	}
	return repos
}

// fetchAccessibleRepositories fetches accessible repositories using the resource map service
func (v *Validator) fetchAccessibleRepositories() ([]string, error) {
	repos, err := GetAllAccessibleRepos(v.userEmail)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch accessible repositories: %w", err)
	}

	// Convert repository structs to normalized URL format
	repoURLs := make([]string, 0, len(repos))
	for _, repo := range repos {
		repoURL := fmt.Sprintf("github.com/%s/%s", repo.GetOrg(), repo.GetRepo())
		repoURLs = append(repoURLs, repoURL)
	}

	return repoURLs, nil
}

// normalizeRepositoryURL converts various GitHub URL formats to a consistent format
// Handles: https://github.com/owner/repo, github.com/owner/repo, owner/repo
func normalizeRepositoryURL(repoURL string) (string, error) {
	if repoURL == "" {
		return "", fmt.Errorf("repository URL cannot be empty")
	}

	// Handle full URLs
	if strings.HasPrefix(repoURL, "http://") || strings.HasPrefix(repoURL, "https://") {
		parsedURL, err := url.Parse(repoURL)
		if err != nil {
			return "", fmt.Errorf("invalid URL format: %w", err)
		}
		
		// Validate that this is a GitHub URL
		if parsedURL.Host != "github.com" {
			return "", fmt.Errorf("unsupported repository URL format: %s", repoURL)
		}
		
		// Extract the path and remove leading slash
		path := strings.TrimPrefix(parsedURL.Path, "/")
		// Remove .git suffix if present
		path = strings.TrimSuffix(path, ".git")
		
		// Validate path format (should be owner/repo)
		if !isValidRepoPath(path) {
			return "", fmt.Errorf("unsupported repository URL format: %s", repoURL)
		}
		
		return fmt.Sprintf("github.com/%s", path), nil
	}

	// Handle github.com/owner/repo format
	if strings.HasPrefix(repoURL, "github.com/") {
		path := strings.TrimPrefix(repoURL, "github.com/")
		path = strings.TrimSuffix(path, ".git")
		
		// Validate path format (should be owner/repo)
		if !isValidRepoPath(path) {
			return "", fmt.Errorf("unsupported repository URL format: %s", repoURL)
		}
		
		return fmt.Sprintf("github.com/%s", path), nil
	}

	// Handle owner/repo format
	if strings.Count(repoURL, "/") == 1 {
		path := strings.TrimSuffix(repoURL, ".git")
		
		// Validate path format (should be owner/repo)
		if !isValidRepoPath(path) {
			return "", fmt.Errorf("unsupported repository URL format: %s", repoURL)
		}
		
		return fmt.Sprintf("github.com/%s", path), nil
	}

	return "", fmt.Errorf("unsupported repository URL format: %s", repoURL)
}

// isValidRepoPath validates that a path is in the format owner/repo
func isValidRepoPath(path string) bool {
	if path == "" {
		return false
	}
	
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		return false
	}
	
	// Both owner and repo name should be non-empty
	return parts[0] != "" && parts[1] != ""
}
