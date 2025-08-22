
# GitHub MCP Server Enhancement: User Email Initialization and Access Validator

## Overview

This document outlines the execution plan for implementing the following changes to the GitHub MCP Server:

1. **User Email Initialization**: Initialize the MCP with a user email parameter
2. **Environment Variable Token Fallback**: If token is not provided in input, try to fetch it from environment variables
3. **Access Validator Component**: Implement repository access validation based on user email

## Current Architecture Analysis

### Authentication Flow
- Currently uses `GITHUB_PERSONAL_ACCESS_TOKEN` environment variable exclusively
- Token is validated in [`cmd/github-mcp-server/main.go:34-37`](cmd/github-mcp-server/main.go:34)
- Client initialization happens in [`internal/ghmcp/server.go:57-78`](internal/ghmcp/server.go:57)
- REST and GraphQL clients are created with the token

### Toolset Architecture
- Tools are organized into toolsets (repos, issues, actions, etc.) in [`pkg/github/tools.go`](pkg/github/tools.go:19)
- Each tool receives a `GetClientFn` that provides authenticated GitHub clients
- Repository operations are scattered across multiple toolsets but primarily in:
  - `repos` toolset: [`GetFileContents`](pkg/github/repositories.go), [`ListCommits`](pkg/github/repositories.go), etc.
  - `actions` toolset: workflow operations
  - `pull_requests` toolset: PR operations
  - `issues` toolset: issue operations

### Testing Patterns
- Uses [`testify`](pkg/github/helper_test.go:9) for assertions
- Mock HTTP responses with [`mockResponse`](pkg/github/helper_test.go:93)
- Helper functions for MCP request creation: [`createMCPRequest`](pkg/github/helper_test.go:112)

## Implementation Plan

### Phase 1: User Email Initialization

#### 1.1 Configuration Changes
**Files to modify:**
- [`cmd/github-mcp-server/main.go`](cmd/github-mcp-server/main.go)
- [`internal/ghmcp/server.go`](internal/ghmcp/server.go)

**Changes:**
```go
// Add to main.go command flags
rootCmd.PersistentFlags().String("user-email", "", "User email for repository access validation")
_ = viper.BindPFlag("user_email", rootCmd.PersistentFlags().Lookup("user-email"))

// Add to StdioServerConfig
type StdioServerConfig struct {
    // ... existing fields ...
    UserEmail string
}

// Add to MCPServerConfig  
type MCPServerConfig struct {
    // ... existing fields ...
    UserEmail string
}
```

#### 1.2 Environment Variable Integration
**Location:** [`cmd/github-mcp-server/main.go:34-37`](cmd/github-mcp-server/main.go:34)

**Current:**
```go
token := viper.GetString("personal_access_token")
if token == "" {
    return errors.New("GITHUB_PERSONAL_ACCESS_TOKEN not set")
}
```

**New Logic:**
```go
userEmail := viper.GetString("user_email")
if userEmail == "" {
    return errors.New("USER_EMAIL not provided in input or environment variable GITHUB_USER_EMAIL")
}

token := viper.GetString("personal_access_token")
if token == "" {
    // Fallback to environment variable
    token = os.Getenv("GITHUB_PERSONAL_ACCESS_TOKEN")
    if token == "" {
        return errors.New("GITHUB_PERSONAL_ACCESS_TOKEN not provided in input or environment")
    }
}
```

### Phase 2: Access Validator Component

#### 2.1 Create Access Validator Package
**New file:** `pkg/access/validator.go`

```go
package access

import (
    "context"
    "fmt"
    "net/url"
    "strings"
    "sync"
    
    "github.com/google/go-github/v74/github"
)

type Validator struct {
    client            *github.Client
    userEmail         string
    accessibleRepos   map[string]struct{}
    mu                sync.RWMutex
    initialized       bool
}

func NewValidator(client *github.Client, userEmail string) *Validator {
    return &Validator{
        client:          client,
        userEmail:       userEmail,
        accessibleRepos: make(map[string]struct{}),
    }
}

func (v *Validator) Initialize(ctx context.Context) error {
    // Dummy implementation - process user email and store accessible repos
    // This would normally call GitHub APIs to get user's accessible repositories
    repos, err := v.fetchAccessibleRepositories(ctx)
    if err != nil {
        return fmt.Errorf("failed to fetch accessible repositories: %w", err)
    }
    
    v.mu.Lock()
    defer v.mu.Unlock()
    
    for _, repo := range repos {
        repoKey := fmt.Sprintf("%s/%s", repo.Owner, repo.Name)
        v.accessibleRepos[repoKey] = struct{}{}
    }
    
    v.initialized = true
    return nil
}

func (v *Validator) HasRepositoryAccess(repoURL string) bool {
    v.mu.RLock()
    defer v.mu.RUnlock()
    
    if !v.initialized {
        return false
    }
    
    repoKey := v.extractRepoKey(repoURL)
    _, exists := v.accessibleRepos[repoKey]
    return exists
}

func (v *Validator) extractRepoKey(repoURL string) string {
    // Handle various URL formats:
    // - https://github.com/owner/repo
    // - owner/repo
    // - owner/repo/path/to/file
    
    if strings.HasPrefix(repoURL, "https://") {
        u, err := url.Parse(repoURL)
        if err != nil {
            return ""
        }
        parts := strings.Split(strings.Trim(u.Path, "/"), "/")
        if len(parts) >= 2 {
            return fmt.Sprintf("%s/%s", parts[0], parts[1])
        }
    }
    
    parts := strings.Split(repoURL, "/")
    if len(parts) >= 2 {
        return fmt.Sprintf("%s/%s", parts[0], parts[1])
    }
    
    return ""
}

type Repository struct {
    Owner string
    Name  string
}

func (v *Validator) fetchAccessibleRepositories(ctx context.Context) ([]Repository, error) {
    // Dummy implementation for now
    // In real implementation, this would:
    // 1. Parse user email to extract username/organization info
    // 2. Call GitHub APIs to get user's accessible repositories
    // 3. Handle pagination
    // 4. Cache results appropriately
    
    // For now, return dummy data based on email processing
    emailParts := strings.Split(v.userEmail, "@")
    if len(emailParts) == 0 {
        return nil, fmt.Errorf("invalid email format")
    }
    
    username := emailParts[0]
    
    // Dummy repos based on username
    return []Repository{
        {Owner: username, Name: "repo1"},
        {Owner: username, Name: "repo2"},
        {Owner: "public-org", Name: "public-repo"},
    }, nil
}
```

#### 2.2 Create Access Validator Tests
**New file:** `pkg/access/validator_test.go`

```go
package access

import (
    "context"
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
    assert.Equal(t, 0, len(validator.accessibleRepos))
}

func TestValidator_Initialize(t *testing.T) {
    client := github.NewClient(nil)
    userEmail := "testuser@example.com"
    
    validator := NewValidator(client, userEmail)
    
    err := validator.Initialize(context.Background())
    require.NoError(t, err)
    
    assert.True(t, validator.initialized)
    
    // Check that dummy repos were added
    assert.True(t, validator.HasRepositoryAccess("testuser/repo1"))
    assert.True(t, validator.HasRepositoryAccess("testuser/repo2"))
    assert.True(t, validator.HasRepositoryAccess("public-org/public-repo"))
    assert.False(t, validator.HasRepositoryAccess("unauthorized/repo"))
}

func TestValidator_HasRepositoryAccess(t *testing.T) {
    tests := []struct {
        name     string
        repoURL  string
        expected bool
    }{
        {
            name:     "accessible repo with full URL",
            repoURL:  "https://github.com/testuser/repo1",
            expected: true,
        },
        {
            name:     "accessible repo with short format",
            repoURL:  "testuser/repo2",
            expected: true,
        },
        {
            name:     "accessible repo with file path",
            repoURL:  "public-org/public-repo/path/to/file.go",
            expected: true,
        },
        {
            name:     "unauthorized repo",
            repoURL:  "unauthorized/repo",
            expected: false,
        },
        {
            name:     "invalid URL format",
            repoURL:  "invalid-format",
            expected: false,
        },
    }
    
    client := github.NewClient(nil)
    validator := NewValidator(client, "testuser@example.com")
    require.NoError(t, validator.Initialize(context.Background()))
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := validator.HasRepositoryAccess(tt.repoURL)
            assert.Equal(t, tt.expected, result)
        })
    }
}

func TestValidator_extractRepoKey(t *testing.T) {
    validator := &Validator{}
    
    tests := []struct {
        name     string
        repoURL  string
        expected string
    }{
        {
            name:     "HTTPS URL",
            repoURL:  "https://github.com/owner/repo",
            expected: "owner/repo",
        },
        {
            name:     "HTTPS URL with path",
            repoURL:  "https://github.com/owner/repo/blob/main/file.go",
            expected: "owner/repo",
        },
        {
            name:     "Short format",
            repoURL:  "owner/repo",
            expected: "owner/repo",
        },
        {
            name:     "Short format with path",
            repoURL:  "owner/repo/path/to/file",
            expected: "owner/repo",
        },
        {
            name:     "Invalid format",
            repoURL:  "invalid",
            expected: "",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := validator.extractRepoKey(tt.repoURL)
            assert.Equal(t, tt.expected, result)
        })
    }
}

func TestValidator_HasRepositoryAccess_NotInitialized(t *testing.T) {
    client := github.NewClient(nil)
    validator := NewValidator(client, "test@example.com")
    
    // Should return false if not initialized
    result := validator.HasRepositoryAccess("owner/repo")
    assert.False(t, result)
}
```

### Phase 3: Integration with Existing Tools

#### 3.1 Modify Server Initialization
**File:** [`internal/ghmcp/server.go`](internal/ghmcp/server.go:57)

```go
func NewMCPServer(cfg MCPServerConfig) (*server.MCPServer, error) {
    // ... existing code ...
    
    // Initialize access validator - BLOCK SERVER STARTUP
    var accessValidator *access.Validator
    if cfg.UserEmail != "" {
        accessValidator = access.NewValidator(restClient, cfg.UserEmail)
        
        // Initialize synchronously and block server startup if it fails
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        
        if err := accessValidator.Initialize(ctx); err != nil {
            return nil, fmt.Errorf("failed to initialize access validator for user %s: %w", cfg.UserEmail, err)
        }
        
        slog.Info("access validator initialized successfully", "userEmail", cfg.UserEmail)
    } else {
        return nil, fmt.Errorf("user email is required for MCP server initialization")
    }
    
    // ... rest of existing code ...
    
    // Pass access validator to toolset creation
    tsg := github.DefaultToolsetGroup(cfg.ReadOnly, getClient, getGQLClient, getRawClient, cfg.Translator, cfg.ContentWindowSize, accessValidator)
    
    // ... rest of existing code ...
}
```

#### 3.2 Integrate Access Validation into Tools
**Files to modify:**
- [`pkg/github/tools.go`](pkg/github/tools.go:19) - Add access validator parameter
- [`pkg/github/repositories.go`](pkg/github/repositories.go) - Repository tools
- [`pkg/github/actions.go`](pkg/github/actions.go) - Actions tools
- [`pkg/github/pullrequests.go`](pkg/github/pullrequests.go) - PR tools
- [`pkg/github/issues.go`](pkg/github/issues.go) - Issue tools

**Example integration in `GetFileContents`:**

```go
func GetFileContents(getClient GetClientFn, getRawClient raw.GetRawClientFn, accessValidator *access.Validator, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
    return mcp.NewTool("get_file_contents",
        // ... existing tool definition ...
    ),
    func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
        owner, err := RequiredParam[string](request, "owner")
        if err != nil {
            return mcp.NewToolResultError(err.Error()), nil
        }
        repo, err := RequiredParam[string](request, "repo")
        if err != nil {
            return mcp.NewToolResultError(err.Error()), nil
        }
        
        // Validate repository access
        if accessValidator != nil {
            repoURL := fmt.Sprintf("%s/%s", owner, repo)
            if !accessValidator.HasRepositoryAccess(repoURL) {
                return mcp.NewToolResultError(fmt.Sprintf("Access denied to repository %s", repoURL)), nil
            }
        }
        
        // ... rest of existing implementation ...
    }
}
```

#### 3.3 Update Tool Registration
**File:** [`pkg/github/tools.go:19`](pkg/github/tools.go:19)

```go
func DefaultToolsetGroup(readOnly bool, getClient GetClientFn, getGQLClient GetGQLClientFn, getRawClient raw.GetRawClientFn, t translations.TranslationHelperFunc, contentWindowSize int, accessValidator *access.Validator) *toolsets.ToolsetGroup {
    // ... existing code ...
    
    repos := toolsets.NewToolset("repos", "GitHub Repository related tools").
        AddReadTools(
            toolsets.NewServerTool(SearchRepositories(getClient, accessValidator, t)),
            toolsets.NewServerTool(GetFileContents(getClient, getRawClient, accessValidator, t)),
            toolsets.NewServerTool(ListCommits(getClient, accessValidator, t)),
            // ... other tools with access validator ...
        )
    
    // ... similar updates for other toolsets ...
}
```

### Phase 4: Testing Strategy

#### 4.1 Unit Tests for Access Validator
**Files:**
- `pkg/access/validator_test.go` (already outlined above)

#### 4.2 Integration Tests for Tools
**Files to create/modify:**
- `pkg/github/repositories_access_test.go`
- `pkg/github/actions_access_test.go`
- `pkg/github/pullrequests_access_test.go`
- `pkg/github/issues_access_test.go`

**Example test for repository access:**

```go
func TestGetFileContents_AccessValidation(t *testing.T) {
    tests := []struct {
        name          string
        userEmail     string
        owner         string
        repo          string
        expectAccess  bool
        expectError   bool
    }{
        {
            name:         "authorized repository",
            userEmail:    "testuser@example.com",
            owner:        "testuser",
            repo:         "repo1",
            expectAccess: true,
            expectError:  false,
        },
        {
            name:         "unauthorized repository",
            userEmail:    "testuser@example.com",
            owner:        "unauthorized",
            repo:         "repo",
            expectAccess: false,
            expectError:  true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            client := github.NewClient(nil)
            validator := access.NewValidator(client, tt.userEmail)
            require.NoError(t, validator.Initialize(context.Background()))
            
            tool, handler := GetFileContents(
                func(ctx context.Context) (*github.Client, error) { return client, nil },
                func(ctx context.Context) (*raw.Client, error) { return nil, nil },
                validator,
                func(key, defaultValue string, args ...interface{}) string { return defaultValue },
            )
            
            request := createMCPRequest(map[string]interface{}{
                "owner": tt.owner,
                "repo":  tt.repo,
                "path":  "README.md",
            })
            
            result, err := handler(context.Background(), request)
            
            if tt.expectError {
                require.NoError(t, err)
                errorResult := getErrorResult(t, result)
                assert.Contains(t, errorResult.Text, "Access denied")
            } else {
                // Test would continue with successful case
                // (mock HTTP responses, etc.)
            }
        })
    }
}
```

#### 4.3 End-to-End Tests
**File:** `e2e/access_validation_test.go`

Tests to verify complete workflow with user email initialization and access validation.

### Phase 5: Documentation Updates

#### 5.1 README Updates
**File:** [`README.md`](README.md)

Add documentation for:
- User email configuration
- Environment variable fallback behavior
- Access validation behavior

#### 5.2 Configuration Examples
Update JSON configuration examples to include user email:

```json
{
  "mcp": {
    "inputs": [
      {
        "type": "promptString",
        "id": "user_email",
        "description": "User Email for Repository Access",
        "password": false
      },
      {
        "type": "promptString",
        "id": "github_token",
        "description": "GitHub Personal Access Token",
        "password": true
      }
    ],
    "servers": {
      "github": {
        "command": "docker",
        "args": [
          "run", "-i", "--rm",
          "-e", "GITHUB_PERSONAL_ACCESS_TOKEN",
          "-e", "GITHUB_USER_EMAIL",
          "ghcr.io/github/github-mcp-server",
          "--user-email", "${input:user_email}"
        ],
        "env": {
          "GITHUB_PERSONAL_ACCESS_TOKEN": "${input:github_token}",
          "GITHUB_USER_EMAIL": "${input:user_email}"
        }
      }
    }
  }
}
```

## Repository Operations Requiring Access Validation

### Primary Targets (High Priority)
1. **Repository Tools** (`repos` toolset):
   - [`GetFileContents`](pkg/github/repositories.go) - Critical for file access
   - [`ListCommits`](pkg/github/repositories.go) - Repository history access
   - [`GetCommit`](pkg/github/repositories.go) - Specific commit access
   - [`ListBranches`](pkg/github/repositories.go) - Branch listing
   - [`ListTags`](pkg/github/repositories.go) - Tag listing
   - [`CreateBranch`](pkg/github/repositories.go) - Branch creation (write)

2. **Actions Tools** (`actions` toolset):
   - [`ListWorkflows`](pkg/github/actions.go) - Workflow access
   - [`ListWorkflowRuns`](pkg/github/actions.go) - Run history
   - [`GetWorkflowRun`](pkg/github/actions.go) - Specific run details
   - [`RunWorkflow`](pkg/github/actions.go) - Trigger workflows (write)

3. **Pull Request Tools** (`pull_requests` toolset):
   - [`GetPullRequest`](pkg/github/pullrequests.go) - PR details
   - [`ListPullRequests`](pkg/github/pullrequests.go) - PR listing
   - [`CreatePullRequest`](pkg/github/pullrequests.go) - PR creation (write)

4. **Issue Tools** (`issues` toolset):
   - [`GetIssue`](pkg/github/issues.go) - Issue details
   - [`ListIssues`](pkg/github/issues.go) - Issue listing
   - [`CreateIssue`](pkg/github/issues.go) - Issue creation (write)

### Secondary Targets (Medium Priority)
- Security scanning tools (code scanning, secret scanning, dependabot)
- Notification tools (if repository-specific)

## Implementation Timeline

### Week 1: Foundation
- [ ] Phase 1: User email initialization
- [ ] Phase 2.1: Access validator component
- [ ] Basic unit tests for access validator

### Week 2: Integration
- [ ] Phase 3.1: Server integration
- [ ] Phase 3.2: Tool integration (high priority tools)
- [ ] Phase 4.1-4.2: Unit and integration tests

### Week 3: Expansion & Testing
- [ ] Complete tool integration (remaining tools)
- [ ] Phase 4.3: End-to-end tests
- [ ] Phase 5: Documentation updates

### Week 4: Validation & Polish
- [ ] Comprehensive testing
- [ ] Performance validation
- [ ] Documentation review
- [ ] Code review and refinements

## Risk Mitigation

### Performance Concerns
- **Issue**: Access validation on every repository operation
- **Mitigation**:
  - Cache repository access results
  - Provide bypass option for trusted environments
  - Optimize GitHub API calls during initialization

### Backward Compatibility
- **Issue**: Breaking changes to existing configurations
- **Mitigation**:
  - **BREAKING CHANGE**: User email is now required for server initialization
  - Clear migration documentation for existing users
  - Explicit error messages when user email is missing

### Error Handling
- **Issue**: Network failures during access validation initialization
- **Mitigation**:
  - **BLOCKING**: Server initialization fails if access validator cannot initialize
  - 30-second timeout for initialization with clear error messages
  - Retry mechanisms with exponential backoff for GitHub API calls
  - Detailed error messages indicating specific failure reasons

## Success Criteria

1. **Functional Requirements**:
   - ✅ MCP initializes with user email parameter
   - ✅ Token fallback from environment variables works
   - ✅ Access validator correctly validates repository access
   - ✅ All repository operations respect access validation

2. **Quality Requirements**:
   - ✅ >90% test coverage for new components
   - ✅ No performance degradation >100ms per operation
   - ✅ Backward compatibility maintained
   - ✅ Clear documentation and examples

3. **Integration Requirements**:
   - ✅ Works with existing MCP hosts (VS Code, Claude, etc.)
   - ✅ Docker configuration updated
   - ✅ Environment variable configuration works
   - ✅ Error messages are user-friendly

This execution plan provides a comprehensive roadmap for implementing the requested features while maintaining code quality, backward compatibility, and system reliability.
