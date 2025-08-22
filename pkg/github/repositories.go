package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/github/github-mcp-server/pkg/access"
	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/raw"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v74/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func GetCommit(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_commit",
			mcp.WithDescription(t("TOOL_GET_COMMITS_DESCRIPTION", "Get details for a commit from a GitHub repository")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_COMMITS_USER_TITLE", "Get commit details"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithString("sha",
				mcp.Required(),
				mcp.Description("Commit SHA, branch name, or tag name"),
			),
			WithPagination(),
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
			sha, err := RequiredParam[string](request, "sha")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			pagination, err := OptionalPaginationParams(request)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			opts := &github.ListOptions{
				Page:    pagination.Page,
				PerPage: pagination.PerPage,
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			commit, resp, err := client.Repositories.GetCommit(ctx, owner, repo, sha, opts)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to get commit: %s", sha),
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != 200 {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to get commit: %s", string(body))), nil
			}

			r, err := json.Marshal(commit)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// ListCommits creates a tool to get commits of a branch in a repository.
func ListCommits(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_commits",
			mcp.WithDescription(t("TOOL_LIST_COMMITS_DESCRIPTION", "Get list of commits of a branch in a GitHub repository. Returns at least 30 results per page by default, but can return more if specified using the perPage parameter (up to 100).")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_LIST_COMMITS_USER_TITLE", "List commits"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithString("sha",
				mcp.Description("Commit SHA, branch or tag name to list commits of. If not provided, uses the default branch of the repository. If a commit SHA is provided, will list commits up to that SHA."),
			),
			mcp.WithString("author",
				mcp.Description("Author username or email address to filter commits by"),
			),
			WithPagination(),
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
			sha, err := OptionalParam[string](request, "sha")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			author, err := OptionalParam[string](request, "author")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			pagination, err := OptionalPaginationParams(request)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			// Set default perPage to 30 if not provided
			perPage := pagination.PerPage
			if perPage == 0 {
				perPage = 30
			}
			opts := &github.CommitsListOptions{
				SHA:    sha,
				Author: author,
				ListOptions: github.ListOptions{
					Page:    pagination.Page,
					PerPage: perPage,
				},
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			commits, resp, err := client.Repositories.ListCommits(ctx, owner, repo, opts)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to list commits: %s", sha),
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != 200 {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to list commits: %s", string(body))), nil
			}

			r, err := json.Marshal(commits)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// ListBranches creates a tool to list branches in a GitHub repository.
func ListBranches(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_branches",
			mcp.WithDescription(t("TOOL_LIST_BRANCHES_DESCRIPTION", "List branches in a GitHub repository")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_LIST_BRANCHES_USER_TITLE", "List branches"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			WithPagination(),
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
			pagination, err := OptionalPaginationParams(request)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			opts := &github.BranchListOptions{
				ListOptions: github.ListOptions{
					Page:    pagination.Page,
					PerPage: pagination.PerPage,
				},
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			branches, resp, err := client.Repositories.ListBranches(ctx, owner, repo, opts)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to list branches",
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to list branches: %s", string(body))), nil
			}

			r, err := json.Marshal(branches)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}



// GetFileContents creates a tool to get the contents of a file or directory from a GitHub repository.
func GetFileContents(getClient GetClientFn, getRawClient raw.GetRawClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_file_contents",
			mcp.WithDescription(t("TOOL_GET_FILE_CONTENTS_DESCRIPTION", "Get the contents of a file or directory from a GitHub repository")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_FILE_CONTENTS_USER_TITLE", "Get file or directory contents"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner (username or organization)"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithString("path",
				mcp.Description("Path to file/directory (directories must end with a slash '/')"),
				mcp.DefaultString("/"),
			),
			mcp.WithString("ref",
				mcp.Description("Accepts optional git refs such as `refs/tags/{tag}`, `refs/heads/{branch}` or `refs/pull/{pr_number}/head`"),
			),
			mcp.WithString("sha",
				mcp.Description("Accepts optional commit SHA. If specified, it will be used instead of ref"),
			),
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
			path, err := RequiredParam[string](request, "path")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			ref, err := OptionalParam[string](request, "ref")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			sha, err := OptionalParam[string](request, "sha")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return mcp.NewToolResultError("failed to get GitHub client"), nil
			}

			rawOpts, err := resolveGitReference(ctx, client, owner, repo, ref, sha)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to resolve git reference: %s", err)), nil
			}

			// If the path is (most likely) not to be a directory, we will
			// first try to get the raw content from the GitHub raw content API.
			if path != "" && !strings.HasSuffix(path, "/") {
				// First, get file info from Contents API to retrieve SHA
				var fileSHA string
				opts := &github.RepositoryContentGetOptions{Ref: ref}
				fileContent, _, respContents, err := client.Repositories.GetContents(ctx, owner, repo, path, opts)
				if respContents != nil {
					defer func() { _ = respContents.Body.Close() }()
				}
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx,
						"failed to get file SHA",
						respContents,
						err,
					), nil
				}
				if fileContent == nil || fileContent.SHA == nil {
					return mcp.NewToolResultError("file content SHA is nil"), nil
				}
				fileSHA = *fileContent.SHA

				rawClient, err := getRawClient(ctx)
				if err != nil {
					return mcp.NewToolResultError("failed to get GitHub raw content client"), nil
				}
				resp, err := rawClient.GetRawContent(ctx, owner, repo, path, rawOpts)
				if err != nil {
					return mcp.NewToolResultError("failed to get raw repository content"), nil
				}
				defer func() {
					_ = resp.Body.Close()
				}()

				if resp.StatusCode == http.StatusOK {
					// If the raw content is found, return it directly
					body, err := io.ReadAll(resp.Body)
					if err != nil {
						return mcp.NewToolResultError("failed to read response body"), nil
					}
					contentType := resp.Header.Get("Content-Type")

					var resourceURI string
					switch {
					case sha != "":
						resourceURI, err = url.JoinPath("repo://", owner, repo, "sha", sha, "contents", path)
						if err != nil {
							return nil, fmt.Errorf("failed to create resource URI: %w", err)
						}
					case ref != "":
						resourceURI, err = url.JoinPath("repo://", owner, repo, ref, "contents", path)
						if err != nil {
							return nil, fmt.Errorf("failed to create resource URI: %w", err)
						}
					default:
						resourceURI, err = url.JoinPath("repo://", owner, repo, "contents", path)
						if err != nil {
							return nil, fmt.Errorf("failed to create resource URI: %w", err)
						}
					}

					if strings.HasPrefix(contentType, "application") || strings.HasPrefix(contentType, "text") {
						result := mcp.TextResourceContents{
							URI:      resourceURI,
							Text:     string(body),
							MIMEType: contentType,
						}
						// Include SHA in the result metadata
						if fileSHA != "" {
							return mcp.NewToolResultResource(fmt.Sprintf("successfully downloaded text file (SHA: %s)", fileSHA), result), nil
						}
						return mcp.NewToolResultResource("successfully downloaded text file", result), nil
					}

					result := mcp.BlobResourceContents{
						URI:      resourceURI,
						Blob:     base64.StdEncoding.EncodeToString(body),
						MIMEType: contentType,
					}
					// Include SHA in the result metadata
					if fileSHA != "" {
						return mcp.NewToolResultResource(fmt.Sprintf("successfully downloaded binary file (SHA: %s)", fileSHA), result), nil
					}
					return mcp.NewToolResultResource("successfully downloaded binary file", result), nil

				}
			}

			if rawOpts.SHA != "" {
				ref = rawOpts.SHA
			}
			if strings.HasSuffix(path, "/") {
				opts := &github.RepositoryContentGetOptions{Ref: ref}
				_, dirContent, resp, err := client.Repositories.GetContents(ctx, owner, repo, path, opts)
				if err == nil && resp.StatusCode == http.StatusOK {
					defer func() { _ = resp.Body.Close() }()
					r, err := json.Marshal(dirContent)
					if err != nil {
						return mcp.NewToolResultError("failed to marshal response"), nil
					}
					return mcp.NewToolResultText(string(r)), nil
				}
			}

			// The path does not point to a file or directory.
			// Instead let's try to find it in the Git Tree by matching the end of the path.

			// Step 1: Get Git Tree recursively
			tree, resp, err := client.Git.GetTree(ctx, owner, repo, ref, true)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to get git tree",
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			// Step 2: Filter tree for matching paths
			const maxMatchingFiles = 3
			matchingFiles := filterPaths(tree.Entries, path, maxMatchingFiles)
			if len(matchingFiles) > 0 {
				matchingFilesJSON, err := json.Marshal(matchingFiles)
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("failed to marshal matching files: %s", err)), nil
				}
				resolvedRefs, err := json.Marshal(rawOpts)
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("failed to marshal resolved refs: %s", err)), nil
				}
				return mcp.NewToolResultText(fmt.Sprintf("Path did not point to a file or directory, but resolved git ref to %s with possible path matches: %s", resolvedRefs, matchingFilesJSON)), nil
			}

			return mcp.NewToolResultError("Failed to get file contents. The path does not point to a file or directory, or the file does not exist in the repository."), nil
		}
}



// CreateBranch creates a tool to create a new branch.
func CreateBranch(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("create_branch",
			mcp.WithDescription(t("TOOL_CREATE_BRANCH_DESCRIPTION", "Create a new branch in a GitHub repository")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_CREATE_BRANCH_USER_TITLE", "Create branch"),
				ReadOnlyHint: ToBoolPtr(false),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithString("branch",
				mcp.Required(),
				mcp.Description("Name for new branch"),
			),
			mcp.WithString("from_branch",
				mcp.Description("Source branch (defaults to repo default)"),
			),
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
			branch, err := RequiredParam[string](request, "branch")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			fromBranch, err := OptionalParam[string](request, "from_branch")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			// Get the source branch SHA
			var ref *github.Reference

			if fromBranch == "" {
				// Get default branch if from_branch not specified
				repository, resp, err := client.Repositories.Get(ctx, owner, repo)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx,
						"failed to get repository",
						resp,
						err,
					), nil
				}
				defer func() { _ = resp.Body.Close() }()

				fromBranch = *repository.DefaultBranch
			}

			// Get SHA of source branch
			ref, resp, err := client.Git.GetRef(ctx, owner, repo, "refs/heads/"+fromBranch)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to get reference",
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			// Create new branch
			newRef := &github.Reference{
				Ref:    github.Ptr("refs/heads/" + branch),
				Object: &github.GitObject{SHA: ref.Object.SHA},
			}

			createdRef, resp, err := client.Git.CreateRef(ctx, owner, repo, newRef)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to create branch",
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			r, err := json.Marshal(createdRef)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// GetFileContentsWithValidation creates a tool to get the contents of a file or directory from a GitHub repository with access validation.
func GetFileContentsWithValidation(getClient GetClientFn, getRawClient raw.GetRawClientFn, t translations.TranslationHelperFunc, validator *access.Validator) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_file_contents",
			mcp.WithDescription(t("TOOL_GET_FILE_CONTENTS_DESCRIPTION", "Get the contents of a file or directory from a GitHub repository")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_FILE_CONTENTS_USER_TITLE", "Get file or directory contents"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner (username or organization)"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithString("path",
				mcp.Description("Path to file/directory (directories must end with a slash '/')"),
				mcp.DefaultString("/"),
			),
			mcp.WithString("ref",
				mcp.Description("Accepts optional git refs such as `refs/tags/{tag}`, `refs/heads/{branch}` or `refs/pull/{pr_number}/head`"),
			),
			mcp.WithString("sha",
				mcp.Description("Accepts optional commit SHA. If specified, it will be used instead of ref"),
			),
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

			// ACCESS VALIDATION: Check if the repository is accessible
			repoURL := fmt.Sprintf("github.com/%s/%s", owner, repo)
			accessible, err := validator.IsRepositoryAccessible(repoURL)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to validate repository access: %s", err.Error())), nil
			}
			if !accessible {
				return mcp.NewToolResultError(fmt.Sprintf("Access denied: Repository %s/%s is not accessible to the current user", owner, repo)), nil
			}

			path, err := RequiredParam[string](request, "path")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			ref, err := OptionalParam[string](request, "ref")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			sha, err := OptionalParam[string](request, "sha")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return mcp.NewToolResultError("failed to get GitHub client"), nil
			}

			rawOpts, err := resolveGitReference(ctx, client, owner, repo, ref, sha)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to resolve git reference: %s", err)), nil
			}

			// If the path is (most likely) not to be a directory, we will
			// first try to get the raw content from the GitHub raw content API.
			if path != "" && !strings.HasSuffix(path, "/") {
				// First, get file info from Contents API to retrieve SHA
				var fileSHA string
				opts := &github.RepositoryContentGetOptions{Ref: ref}
				fileContent, _, respContents, err := client.Repositories.GetContents(ctx, owner, repo, path, opts)
				if respContents != nil {
					defer func() { _ = respContents.Body.Close() }()
				}
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx,
						"failed to get file SHA",
						respContents,
						err,
					), nil
				}
				if fileContent == nil || fileContent.SHA == nil {
					return mcp.NewToolResultError("file content SHA is nil"), nil
				}
				fileSHA = *fileContent.SHA

				rawClient, err := getRawClient(ctx)
				if err != nil {
					return mcp.NewToolResultError("failed to get GitHub raw content client"), nil
				}
				resp, err := rawClient.GetRawContent(ctx, owner, repo, path, rawOpts)
				if err != nil {
					return mcp.NewToolResultError("failed to get raw repository content"), nil
				}
				defer func() {
					_ = resp.Body.Close()
				}()

				if resp.StatusCode == http.StatusOK {
					// If the raw content is found, return it directly
					body, err := io.ReadAll(resp.Body)
					if err != nil {
						return mcp.NewToolResultError("failed to read response body"), nil
					}
					contentType := resp.Header.Get("Content-Type")

					var resourceURI string
					switch {
					case sha != "":
						resourceURI, err = url.JoinPath("repo://", owner, repo, "sha", sha, "contents", path)
						if err != nil {
							return nil, fmt.Errorf("failed to create resource URI: %w", err)
						}
					case ref != "":
						resourceURI, err = url.JoinPath("repo://", owner, repo, ref, "contents", path)
						if err != nil {
							return nil, fmt.Errorf("failed to create resource URI: %w", err)
						}
					default:
						resourceURI, err = url.JoinPath("repo://", owner, repo, "contents", path)
						if err != nil {
							return nil, fmt.Errorf("failed to create resource URI: %w", err)
						}
					}

					if strings.HasPrefix(contentType, "application") || strings.HasPrefix(contentType, "text") {
						result := mcp.TextResourceContents{
							URI:      resourceURI,
							Text:     string(body),
							MIMEType: contentType,
						}
						// Include SHA in the result metadata
						if fileSHA != "" {
							return mcp.NewToolResultResource(fmt.Sprintf("successfully downloaded text file (SHA: %s)", fileSHA), result), nil
						}
						return mcp.NewToolResultResource("successfully downloaded text file", result), nil
					}

					result := mcp.BlobResourceContents{
						URI:      resourceURI,
						Blob:     base64.StdEncoding.EncodeToString(body),
						MIMEType: contentType,
					}
					// Include SHA in the result metadata
					if fileSHA != "" {
						return mcp.NewToolResultResource(fmt.Sprintf("successfully downloaded binary file (SHA: %s)", fileSHA), result), nil
					}
					return mcp.NewToolResultResource("successfully downloaded binary file", result), nil

				}
			}

			if rawOpts.SHA != "" {
				ref = rawOpts.SHA
			}
			if strings.HasSuffix(path, "/") {
				opts := &github.RepositoryContentGetOptions{Ref: ref}
				_, dirContent, resp, err := client.Repositories.GetContents(ctx, owner, repo, path, opts)
				if err == nil && resp.StatusCode == http.StatusOK {
					defer func() { _ = resp.Body.Close() }()
					r, err := json.Marshal(dirContent)
					if err != nil {
						return mcp.NewToolResultError("failed to marshal response"), nil
					}
					return mcp.NewToolResultText(string(r)), nil
				}
			}

			opts := &github.RepositoryContentGetOptions{Ref: ref}
			file, _, resp, err := client.Repositories.GetContents(ctx, owner, repo, path, opts)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to get file contents",
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			r, err := json.Marshal(file)
			if err != nil {
				return mcp.NewToolResultError("failed to marshal response"), nil
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

func GetCommitWithValidation(getClient GetClientFn, t translations.TranslationHelperFunc, validator *access.Validator) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_commit",
		mcp.WithDescription(t("TOOL_GET_COMMITS_DESCRIPTION", "Get details for a commit from a GitHub repository")),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{
			Title:        t("TOOL_GET_COMMITS_USER_TITLE", "Get commit details"),
			ReadOnlyHint: ToBoolPtr(true),
		}),
		mcp.WithString("owner",
			mcp.Required(),
			mcp.Description("Repository owner"),
		),
		mcp.WithString("repo",
			mcp.Required(),
			mcp.Description("Repository name"),
		),
		mcp.WithString("sha",
			mcp.Required(),
			mcp.Description("Commit SHA, branch name, or tag name"),
		),
		WithPagination(),
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

			// ACCESS VALIDATION: Check if the repository is accessible
			repoURL := fmt.Sprintf("github.com/%s/%s", owner, repo)
			accessible, err := validator.IsRepositoryAccessible(repoURL)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to validate repository access: %s", err.Error())), nil
			}
			if !accessible {
				return mcp.NewToolResultError(fmt.Sprintf("Access denied: Repository %s/%s is not accessible to the current user", owner, repo)), nil
			}

			sha, err := RequiredParam[string](request, "sha")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			pagination, err := OptionalPaginationParams(request)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			opts := &github.ListOptions{
				Page:    pagination.Page,
				PerPage: pagination.PerPage,
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			commit, resp, err := client.Repositories.GetCommit(ctx, owner, repo, sha, opts)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to get commit: %s", sha),
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != 200 {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to get commit: %s", string(body))), nil
			}

			r, err := json.Marshal(commit)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

func ListCommitsWithValidation(getClient GetClientFn, t translations.TranslationHelperFunc, validator *access.Validator) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_commits",
		mcp.WithDescription(t("TOOL_LIST_COMMITS_DESCRIPTION", "Get list of commits of a branch in a GitHub repository. Returns at least 30 results per page by default, but can return more if specified using the perPage parameter (up to 100).")),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{
			Title:        t("TOOL_LIST_COMMITS_USER_TITLE", "List commits"),
			ReadOnlyHint: ToBoolPtr(true),
		}),
		mcp.WithString("owner",
			mcp.Required(),
			mcp.Description("Repository owner"),
		),
		mcp.WithString("repo",
			mcp.Required(),
			mcp.Description("Repository name"),
		),
		mcp.WithString("sha",
			mcp.Description("Commit SHA, branch or tag name to list commits of. If not provided, uses the default branch of the repository. If a commit SHA is provided, will list commits up to that SHA."),
		),
		mcp.WithString("author",
			mcp.Description("Author username or email address to filter commits by"),
		),
		WithPagination(),
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

			// ACCESS VALIDATION: Check if the repository is accessible
			repoURL := fmt.Sprintf("github.com/%s/%s", owner, repo)
			accessible, err := validator.IsRepositoryAccessible(repoURL)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to validate repository access: %s", err.Error())), nil
			}
			if !accessible {
				return mcp.NewToolResultError(fmt.Sprintf("Access denied: Repository %s/%s is not accessible to the current user", owner, repo)), nil
			}

			sha, err := OptionalParam[string](request, "sha")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			author, err := OptionalParam[string](request, "author")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			pagination, err := OptionalPaginationParams(request)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			// Set default perPage to 30 if not provided
			perPage := pagination.PerPage
			if perPage == 0 {
				perPage = 30
			}
			opts := &github.CommitsListOptions{
				SHA:    sha,
				Author: author,
				ListOptions: github.ListOptions{
					Page:    pagination.Page,
					PerPage: perPage,
				},
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			commits, resp, err := client.Repositories.ListCommits(ctx, owner, repo, opts)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to list commits",
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != 200 {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to list commits: %s", string(body))), nil
			}

			r, err := json.Marshal(commits)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

func ListBranchesWithValidation(getClient GetClientFn, t translations.TranslationHelperFunc, validator *access.Validator) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_branches",
		mcp.WithDescription(t("TOOL_LIST_BRANCHES_DESCRIPTION", "List branches in a GitHub repository")),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{
			Title:        t("TOOL_LIST_BRANCHES_USER_TITLE", "List branches"),
			ReadOnlyHint: ToBoolPtr(true),
		}),
		mcp.WithString("owner",
			mcp.Required(),
			mcp.Description("Repository owner"),
		),
		mcp.WithString("repo",
			mcp.Required(),
			mcp.Description("Repository name"),
		),
		WithPagination(),
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

			// ACCESS VALIDATION: Check if the repository is accessible
			repoURL := fmt.Sprintf("github.com/%s/%s", owner, repo)
			accessible, err := validator.IsRepositoryAccessible(repoURL)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to validate repository access: %s", err.Error())), nil
			}
			if !accessible {
				return mcp.NewToolResultError(fmt.Sprintf("Access denied: Repository %s/%s is not accessible to the current user", owner, repo)), nil
			}

			pagination, err := OptionalPaginationParams(request)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			opts := &github.BranchListOptions{
				ListOptions: github.ListOptions{
					Page:    pagination.Page,
					PerPage: pagination.PerPage,
				},
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			branches, resp, err := client.Repositories.ListBranches(ctx, owner, repo, opts)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to list branches",
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to list branches: %s", string(body))), nil
			}

			r, err := json.Marshal(branches)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

func CreateBranchWithValidation(getClient GetClientFn, t translations.TranslationHelperFunc, validator *access.Validator) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("create_branch",
		mcp.WithDescription(t("TOOL_CREATE_BRANCH_DESCRIPTION", "Create a new branch in a GitHub repository")),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{
			Title:        t("TOOL_CREATE_BRANCH_USER_TITLE", "Create branch"),
			ReadOnlyHint: ToBoolPtr(false),
		}),
		mcp.WithString("owner",
			mcp.Required(),
			mcp.Description("Repository owner"),
		),
		mcp.WithString("repo",
			mcp.Required(),
			mcp.Description("Repository name"),
		),
		mcp.WithString("branch",
			mcp.Required(),
			mcp.Description("Name for new branch"),
		),
		mcp.WithString("from_branch",
			mcp.Description("Source branch (defaults to repo default)"),
		),
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

			// ACCESS VALIDATION: Check if the repository is accessible
			repoURL := fmt.Sprintf("github.com/%s/%s", owner, repo)
			accessible, err := validator.IsRepositoryAccessible(repoURL)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to validate repository access: %s", err.Error())), nil
			}
			if !accessible {
				return mcp.NewToolResultError(fmt.Sprintf("Access denied: Repository %s/%s is not accessible to the current user", owner, repo)), nil
			}

			branch, err := RequiredParam[string](request, "branch")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			fromBranch, err := OptionalParam[string](request, "from_branch")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			// Get the source branch SHA
			var ref *github.Reference

			if fromBranch == "" {
				// Get default branch if from_branch not specified
				repository, resp, err := client.Repositories.Get(ctx, owner, repo)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx,
						"failed to get repository",
						resp,
						err,
					), nil
				}
				defer func() { _ = resp.Body.Close() }()

				fromBranch = *repository.DefaultBranch
			}

			// Get SHA of source branch
			ref, resp, err := client.Git.GetRef(ctx, owner, repo, "refs/heads/"+fromBranch)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to get reference",
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			// Create new branch
			newRef := &github.Reference{
				Ref:    github.Ptr("refs/heads/" + branch),
				Object: &github.GitObject{SHA: ref.Object.SHA},
			}

			createdRef, resp, err := client.Git.CreateRef(ctx, owner, repo, newRef)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to create branch",
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			r, err := json.Marshal(createdRef)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}


// ListTags creates a tool to list tags in a GitHub repository.
func ListTags(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_tags",
			mcp.WithDescription(t("TOOL_LIST_TAGS_DESCRIPTION", "List git tags in a GitHub repository")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_LIST_TAGS_USER_TITLE", "List tags"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			WithPagination(),
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
			pagination, err := OptionalPaginationParams(request)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			opts := &github.ListOptions{
				Page:    pagination.Page,
				PerPage: pagination.PerPage,
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			tags, resp, err := client.Repositories.ListTags(ctx, owner, repo, opts)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to list tags",
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to list tags: %s", string(body))), nil
			}

			r, err := json.Marshal(tags)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// GetTag creates a tool to get details about a specific tag in a GitHub repository.
func GetTag(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_tag",
			mcp.WithDescription(t("TOOL_GET_TAG_DESCRIPTION", "Get details about a specific git tag in a GitHub repository")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_TAG_USER_TITLE", "Get tag details"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithString("tag",
				mcp.Required(),
				mcp.Description("Tag name"),
			),
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
			tag, err := RequiredParam[string](request, "tag")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			// First get the tag reference
			ref, resp, err := client.Git.GetRef(ctx, owner, repo, "refs/tags/"+tag)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to get tag reference",
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to get tag reference: %s", string(body))), nil
			}

			// Then get the tag object
			tagObj, resp, err := client.Git.GetTag(ctx, owner, repo, *ref.Object.SHA)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to get tag object",
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to get tag object: %s", string(body))), nil
			}

			r, err := json.Marshal(tagObj)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// ListReleases creates a tool to list releases in a GitHub repository.
func ListReleases(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_releases",
			mcp.WithDescription(t("TOOL_LIST_RELEASES_DESCRIPTION", "List releases in a GitHub repository")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_LIST_RELEASES_USER_TITLE", "List releases"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			WithPagination(),
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
			pagination, err := OptionalPaginationParams(request)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			opts := &github.ListOptions{
				Page:    pagination.Page,
				PerPage: pagination.PerPage,
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			releases, resp, err := client.Repositories.ListReleases(ctx, owner, repo, opts)
			if err != nil {
				return nil, fmt.Errorf("failed to list releases: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to list releases: %s", string(body))), nil
			}

			r, err := json.Marshal(releases)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// GetLatestRelease creates a tool to get the latest release in a GitHub repository.
func GetLatestRelease(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_latest_release",
			mcp.WithDescription(t("TOOL_GET_LATEST_RELEASE_DESCRIPTION", "Get the latest release in a GitHub repository")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_LATEST_RELEASE_USER_TITLE", "Get latest release"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
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

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			release, resp, err := client.Repositories.GetLatestRelease(ctx, owner, repo)
			if err != nil {
				return nil, fmt.Errorf("failed to get latest release: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to get latest release: %s", string(body))), nil
			}

			r, err := json.Marshal(release)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

func GetReleaseByTag(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_release_by_tag",
			mcp.WithDescription(t("TOOL_GET_RELEASE_BY_TAG_DESCRIPTION", "Get a specific release by its tag name in a GitHub repository")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_RELEASE_BY_TAG_USER_TITLE", "Get a release by tag name"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithString("tag",
				mcp.Required(),
				mcp.Description("Tag name (e.g., 'v1.0.0')"),
			),
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
			tag, err := RequiredParam[string](request, "tag")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			release, resp, err := client.Repositories.GetReleaseByTag(ctx, owner, repo, tag)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to get release by tag: %s", tag),
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to get release by tag: %s", string(body))), nil
			}

			r, err := json.Marshal(release)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// filterPaths filters the entries in a GitHub tree to find paths that
// match the given suffix.
// maxResults limits the number of results returned to first maxResults entries,
// a maxResults of -1 means no limit.
// It returns a slice of strings containing the matching paths.
// Directories are returned with a trailing slash.
func filterPaths(entries []*github.TreeEntry, path string, maxResults int) []string {
	// Remove trailing slash for matching purposes, but flag whether we
	// only want directories.
	dirOnly := false
	if strings.HasSuffix(path, "/") {
		dirOnly = true
		path = strings.TrimSuffix(path, "/")
	}

	matchedPaths := []string{}
	for _, entry := range entries {
		if len(matchedPaths) == maxResults {
			break // Limit the number of results to maxResults
		}
		if dirOnly && entry.GetType() != "tree" {
			continue // Skip non-directory entries if dirOnly is true
		}
		entryPath := entry.GetPath()
		if entryPath == "" {
			continue // Skip empty paths
		}
		if strings.HasSuffix(entryPath, path) {
			if entry.GetType() == "tree" {
				entryPath += "/" // Return directories with a trailing slash
			}
			matchedPaths = append(matchedPaths, entryPath)
		}
	}
	return matchedPaths
}

// resolveGitReference takes a user-provided ref and sha and resolves them into a
// definitive commit SHA and its corresponding fully-qualified reference.
//
// The resolution logic follows a clear priority:
//
//  1. If a specific commit `sha` is provided, it takes precedence and is used directly,
//     and all reference resolution is skipped.
//
//  2. If no `sha` is provided, the function resolves the `ref`
//     string into a fully-qualified format (e.g., "refs/heads/main") by trying
//     the following steps in order:
//     a). **Empty Ref:** If `ref` is empty, the repository's default branch is used.
//     b). **Fully-Qualified:** If `ref` already starts with "refs/", it's considered fully
//     qualified and used as-is.
//     c). **Partially-Qualified:** If `ref` starts with "heads/" or "tags/", it is
//     prefixed with "refs/" to make it fully-qualified.
//     d). **Short Name:** Otherwise, the `ref` is treated as a short name. The function
//     first attempts to resolve it as a branch ("refs/heads/<ref>"). If that
//     returns a 404 Not Found error, it then attempts to resolve it as a tag
//     ("refs/tags/<ref>").
//
//  3. **Final Lookup:** Once a fully-qualified ref is determined, a final API call
//     is made to fetch that reference's definitive commit SHA.
//
// Any unexpected (non-404) errors during the resolution process are returned
// immediately. All API errors are logged with rich context to aid diagnostics.
func resolveGitReference(ctx context.Context, githubClient *github.Client, owner, repo, ref, sha string) (*raw.ContentOpts, error) {
	// 1) If SHA explicitly provided, it's the highest priority.
	if sha != "" {
		return &raw.ContentOpts{Ref: "", SHA: sha}, nil
	}

	originalRef := ref // Keep original ref for clearer error messages down the line.

	// 2) If no SHA is provided, we try to resolve the ref into a fully-qualified format.
	var reference *github.Reference
	var resp *github.Response
	var err error

	switch {
	case originalRef == "":
		// 2a) If ref is empty, determine the default branch.
		repoInfo, resp, err := githubClient.Repositories.Get(ctx, owner, repo)
		if err != nil {
			_, _ = ghErrors.NewGitHubAPIErrorToCtx(ctx, "failed to get repository info", resp, err)
			return nil, fmt.Errorf("failed to get repository info: %w", err)
		}
		ref = fmt.Sprintf("refs/heads/%s", repoInfo.GetDefaultBranch())
	case strings.HasPrefix(originalRef, "refs/"):
		// 2b) Already fully qualified. The reference will be fetched at the end.
	case strings.HasPrefix(originalRef, "heads/") || strings.HasPrefix(originalRef, "tags/"):
		// 2c) Partially qualified. Make it fully qualified.
		ref = "refs/" + originalRef
	default:
		// 2d) It's a short name, so we try to resolve it to either a branch or a tag.
		branchRef := "refs/heads/" + originalRef
		reference, resp, err = githubClient.Git.GetRef(ctx, owner, repo, branchRef)

		if err == nil {
			ref = branchRef // It's a branch.
		} else {
			// The branch lookup failed. Check if it was a 404 Not Found error.
			ghErr, isGhErr := err.(*github.ErrorResponse)
			if isGhErr && ghErr.Response.StatusCode == http.StatusNotFound {
				tagRef := "refs/tags/" + originalRef
				reference, resp, err = githubClient.Git.GetRef(ctx, owner, repo, tagRef)
				if err == nil {
					ref = tagRef // It's a tag.
				} else {
					// The tag lookup also failed. Check if it was a 404 Not Found error.
					ghErr2, isGhErr2 := err.(*github.ErrorResponse)
					if isGhErr2 && ghErr2.Response.StatusCode == http.StatusNotFound {
						return nil, fmt.Errorf("could not resolve ref %q as a branch or a tag", originalRef)
					}
					// The tag lookup failed for a different reason.
					_, _ = ghErrors.NewGitHubAPIErrorToCtx(ctx, "failed to get reference (tag)", resp, err)
					return nil, fmt.Errorf("failed to get reference for tag '%s': %w", originalRef, err)
				}
			} else {
				// The branch lookup failed for a different reason.
				_, _ = ghErrors.NewGitHubAPIErrorToCtx(ctx, "failed to get reference (branch)", resp, err)
				return nil, fmt.Errorf("failed to get reference for branch '%s': %w", originalRef, err)
			}
		}
	}

	if reference == nil {
		reference, resp, err = githubClient.Git.GetRef(ctx, owner, repo, ref)
		if err != nil {
			_, _ = ghErrors.NewGitHubAPIErrorToCtx(ctx, "failed to get final reference", resp, err)
			return nil, fmt.Errorf("failed to get final reference for %q: %w", ref, err)
		}
	}

	sha = reference.GetObject().GetSHA()
	return &raw.ContentOpts{Ref: ref, SHA: sha}, nil
}
