package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/go-viper/mapstructure/v2"
	"github.com/google/go-github/v74/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/shurcooL/githubv4"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/translations"
)

// GetPullRequest creates a tool to get details of a specific pull request.
func GetPullRequest(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("get_pull_request",
			mcp.WithDescription(t("TOOL_GET_PULL_REQUEST_DESCRIPTION", "Get details of a specific pull request in a GitHub repository.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_PULL_REQUEST_USER_TITLE", "Get pull request details"),
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
			mcp.WithNumber("pullNumber",
				mcp.Required(),
				mcp.Description("Pull request number"),
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
			pullNumber, err := RequiredInt(request, "pullNumber")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			pr, resp, err := client.PullRequests.Get(ctx, owner, repo, pullNumber)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to get pull request",
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
				return mcp.NewToolResultError(fmt.Sprintf("failed to get pull request: %s", string(body))), nil
			}

			r, err := json.Marshal(pr)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// CreatePullRequest creates a tool to create a new pull request.
func CreatePullRequest(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("create_pull_request",
			mcp.WithDescription(t("TOOL_CREATE_PULL_REQUEST_DESCRIPTION", "Create a new pull request in a GitHub repository.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_CREATE_PULL_REQUEST_USER_TITLE", "Open new pull request"),
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
			mcp.WithString("title",
				mcp.Required(),
				mcp.Description("PR title"),
			),
			mcp.WithString("body",
				mcp.Description("PR description"),
			),
			mcp.WithString("head",
				mcp.Required(),
				mcp.Description("Branch containing changes"),
			),
			mcp.WithString("base",
				mcp.Required(),
				mcp.Description("Branch to merge into"),
			),
			mcp.WithBoolean("draft",
				mcp.Description("Create as draft PR"),
			),
			mcp.WithBoolean("maintainer_can_modify",
				mcp.Description("Allow maintainer edits"),
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
			title, err := RequiredParam[string](request, "title")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			head, err := RequiredParam[string](request, "head")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			base, err := RequiredParam[string](request, "base")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			body, err := OptionalParam[string](request, "body")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			draft, err := OptionalParam[bool](request, "draft")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			maintainerCanModify, err := OptionalParam[bool](request, "maintainer_can_modify")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			newPR := &github.NewPullRequest{
				Title: github.Ptr(title),
				Head:  github.Ptr(head),
				Base:  github.Ptr(base),
			}

			if body != "" {
				newPR.Body = github.Ptr(body)
			}

			newPR.Draft = github.Ptr(draft)
			newPR.MaintainerCanModify = github.Ptr(maintainerCanModify)

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			pr, resp, err := client.PullRequests.Create(ctx, owner, repo, newPR)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to create pull request",
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusCreated {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to create pull request: %s", string(body))), nil
			}

			r, err := json.Marshal(pr)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}


// ListPullRequests creates a tool to list and filter repository pull requests.
func ListPullRequests(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("list_pull_requests",
			mcp.WithDescription(t("TOOL_LIST_PULL_REQUESTS_DESCRIPTION", "List pull requests in a GitHub repository. If the user specifies an author, then DO NOT use this tool and use the search_pull_requests tool instead.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_LIST_PULL_REQUESTS_USER_TITLE", "List pull requests"),
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
			mcp.WithString("state",
				mcp.Description("Filter by state"),
				mcp.Enum("open", "closed", "all"),
			),
			mcp.WithString("head",
				mcp.Description("Filter by head user/org and branch"),
			),
			mcp.WithString("base",
				mcp.Description("Filter by base branch"),
			),
			mcp.WithString("sort",
				mcp.Description("Sort by"),
				mcp.Enum("created", "updated", "popularity", "long-running"),
			),
			mcp.WithString("direction",
				mcp.Description("Sort direction"),
				mcp.Enum("asc", "desc"),
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
			state, err := OptionalParam[string](request, "state")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			head, err := OptionalParam[string](request, "head")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			base, err := OptionalParam[string](request, "base")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			sort, err := OptionalParam[string](request, "sort")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			direction, err := OptionalParam[string](request, "direction")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			pagination, err := OptionalPaginationParams(request)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			opts := &github.PullRequestListOptions{
				State:     state,
				Head:      head,
				Base:      base,
				Sort:      sort,
				Direction: direction,
				ListOptions: github.ListOptions{
					PerPage: pagination.PerPage,
					Page:    pagination.Page,
				},
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			prs, resp, err := client.PullRequests.List(ctx, owner, repo, opts)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to list pull requests",
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
				return mcp.NewToolResultError(fmt.Sprintf("failed to list pull requests: %s", string(body))), nil
			}

			r, err := json.Marshal(prs)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}


// SearchPullRequests creates a tool to search for pull requests.
func SearchPullRequests(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("search_pull_requests",
			mcp.WithDescription(t("TOOL_SEARCH_PULL_REQUESTS_DESCRIPTION", "Search for pull requests in GitHub repositories using issues search syntax already scoped to is:pr")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_SEARCH_PULL_REQUESTS_USER_TITLE", "Search pull requests"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("Search query using GitHub pull request search syntax"),
			),
			mcp.WithString("owner",
				mcp.Description("Optional repository owner. If provided with repo, only pull requests for this repository are listed."),
			),
			mcp.WithString("repo",
				mcp.Description("Optional repository name. If provided with owner, only pull requests for this repository are listed."),
			),
			mcp.WithString("sort",
				mcp.Description("Sort field by number of matches of categories, defaults to best match"),
				mcp.Enum(
					"comments",
					"reactions",
					"reactions-+1",
					"reactions--1",
					"reactions-smile",
					"reactions-thinking_face",
					"reactions-heart",
					"reactions-tada",
					"interactions",
					"created",
					"updated",
				),
			),
			mcp.WithString("order",
				mcp.Description("Sort order"),
				mcp.Enum("asc", "desc"),
			),
			WithPagination(),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return searchHandler(ctx, getClient, request, "pr", "failed to search pull requests")
		}
}

// GetPullRequestFiles creates a tool to get the list of files changed in a pull request.
func GetPullRequestFiles(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("get_pull_request_files",
			mcp.WithDescription(t("TOOL_GET_PULL_REQUEST_FILES_DESCRIPTION", "Get the files changed in a specific pull request.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_PULL_REQUEST_FILES_USER_TITLE", "Get pull request files"),
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
			mcp.WithNumber("pullNumber",
				mcp.Required(),
				mcp.Description("Pull request number"),
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
			pullNumber, err := RequiredInt(request, "pullNumber")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			pagination, err := OptionalPaginationParams(request)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			opts := &github.ListOptions{
				PerPage: pagination.PerPage,
				Page:    pagination.Page,
			}
			files, resp, err := client.PullRequests.ListFiles(ctx, owner, repo, pullNumber, opts)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to get pull request files",
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
				return mcp.NewToolResultError(fmt.Sprintf("failed to get pull request files: %s", string(body))), nil
			}

			r, err := json.Marshal(files)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// GetPullRequestStatus creates a tool to get the combined status of all status checks for a pull request.
func GetPullRequestStatus(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("get_pull_request_status",
			mcp.WithDescription(t("TOOL_GET_PULL_REQUEST_STATUS_DESCRIPTION", "Get the status of a specific pull request.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_PULL_REQUEST_STATUS_USER_TITLE", "Get pull request status checks"),
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
			mcp.WithNumber("pullNumber",
				mcp.Required(),
				mcp.Description("Pull request number"),
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
			pullNumber, err := RequiredInt(request, "pullNumber")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			// First get the PR to find the head SHA
			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			pr, resp, err := client.PullRequests.Get(ctx, owner, repo, pullNumber)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to get pull request",
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
				return mcp.NewToolResultError(fmt.Sprintf("failed to get pull request: %s", string(body))), nil
			}

			// Get combined status for the head SHA
			status, resp, err := client.Repositories.GetCombinedStatus(ctx, owner, repo, *pr.Head.SHA, nil)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to get combined status",
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
				return mcp.NewToolResultError(fmt.Sprintf("failed to get combined status: %s", string(body))), nil
			}

			r, err := json.Marshal(status)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}


// GetPullRequestComments creates a tool to get the review comments on a pull request.
func GetPullRequestComments(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("get_pull_request_comments",
			mcp.WithDescription(t("TOOL_GET_PULL_REQUEST_COMMENTS_DESCRIPTION", "Get comments for a specific pull request.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_PULL_REQUEST_COMMENTS_USER_TITLE", "Get pull request comments"),
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
			mcp.WithNumber("pullNumber",
				mcp.Required(),
				mcp.Description("Pull request number"),
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
			pullNumber, err := RequiredInt(request, "pullNumber")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			opts := &github.PullRequestListCommentsOptions{
				ListOptions: github.ListOptions{
					PerPage: 100,
				},
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			comments, resp, err := client.PullRequests.ListComments(ctx, owner, repo, pullNumber, opts)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to get pull request comments",
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
				return mcp.NewToolResultError(fmt.Sprintf("failed to get pull request comments: %s", string(body))), nil
			}

			r, err := json.Marshal(comments)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// GetPullRequestReviews creates a tool to get the reviews on a pull request.
func GetPullRequestReviews(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("get_pull_request_reviews",
			mcp.WithDescription(t("TOOL_GET_PULL_REQUEST_REVIEWS_DESCRIPTION", "Get reviews for a specific pull request.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_PULL_REQUEST_REVIEWS_USER_TITLE", "Get pull request reviews"),
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
			mcp.WithNumber("pullNumber",
				mcp.Required(),
				mcp.Description("Pull request number"),
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
			pullNumber, err := RequiredInt(request, "pullNumber")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			reviews, resp, err := client.PullRequests.ListReviews(ctx, owner, repo, pullNumber, nil)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to get pull request reviews",
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
				return mcp.NewToolResultError(fmt.Sprintf("failed to get pull request reviews: %s", string(body))), nil
			}

			r, err := json.Marshal(reviews)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}



// AddCommentToPendingReview creates a tool to add a comment to a pull request review.
func AddCommentToPendingReview(getGQLClient GetGQLClientFn, t translations.TranslationHelperFunc) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("add_comment_to_pending_review",
			mcp.WithDescription(t("TOOL_ADD_COMMENT_TO_PENDING_REVIEW_DESCRIPTION", "Add review comment to the requester's latest pending pull request review. A pending review needs to already exist to call this (check with the user if not sure).")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_ADD_COMMENT_TO_PENDING_REVIEW_USER_TITLE", "Add review comment to the requester's latest pending pull request review"),
				ReadOnlyHint: ToBoolPtr(false),
			}),
			// Ideally, for performance sake this would just accept the pullRequestReviewID. However, we would need to
			// add a new tool to get that ID for clients that aren't in the same context as the original pending review
			// creation. So for now, we'll just accept the owner, repo and pull number and assume this is adding a comment
			// the latest review from a user, since only one can be active at a time. It can later be extended with
			// a pullRequestReviewID parameter if targeting other reviews is desired:
			// mcp.WithString("pullRequestReviewID",
			// 	mcp.Required(),
			// 	mcp.Description("The ID of the pull request review to add a comment to"),
			// ),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithNumber("pullNumber",
				mcp.Required(),
				mcp.Description("Pull request number"),
			),
			mcp.WithString("path",
				mcp.Required(),
				mcp.Description("The relative path to the file that necessitates a comment"),
			),
			mcp.WithString("body",
				mcp.Required(),
				mcp.Description("The text of the review comment"),
			),
			mcp.WithString("subjectType",
				mcp.Required(),
				mcp.Description("The level at which the comment is targeted"),
				mcp.Enum("FILE", "LINE"),
			),
			mcp.WithNumber("line",
				mcp.Description("The line of the blob in the pull request diff that the comment applies to. For multi-line comments, the last line of the range"),
			),
			mcp.WithString("side",
				mcp.Description("The side of the diff to comment on. LEFT indicates the previous state, RIGHT indicates the new state"),
				mcp.Enum("LEFT", "RIGHT"),
			),
			mcp.WithNumber("startLine",
				mcp.Description("For multi-line comments, the first line of the range that the comment applies to"),
			),
			mcp.WithString("startSide",
				mcp.Description("For multi-line comments, the starting side of the diff that the comment applies to. LEFT indicates the previous state, RIGHT indicates the new state"),
				mcp.Enum("LEFT", "RIGHT"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			var params struct {
				Owner       string
				Repo        string
				PullNumber  int32
				Path        string
				Body        string
				SubjectType string
				Line        *int32
				Side        *string
				StartLine   *int32
				StartSide   *string
			}
			if err := mapstructure.Decode(request.Params.Arguments, &params); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getGQLClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub GQL client: %w", err)
			}

			// First we'll get the current user
			var getViewerQuery struct {
				Viewer struct {
					Login githubv4.String
				}
			}

			if err := client.Query(ctx, &getViewerQuery, nil); err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx,
					"failed to get current user",
					err,
				), nil
			}

			var getLatestReviewForViewerQuery struct {
				Repository struct {
					PullRequest struct {
						Reviews struct {
							Nodes []struct {
								ID    githubv4.ID
								State githubv4.PullRequestReviewState
								URL   githubv4.URI
							}
						} `graphql:"reviews(first: 1, author: $author)"`
					} `graphql:"pullRequest(number: $prNum)"`
				} `graphql:"repository(owner: $owner, name: $name)"`
			}

			vars := map[string]any{
				"author": githubv4.String(getViewerQuery.Viewer.Login),
				"owner":  githubv4.String(params.Owner),
				"name":   githubv4.String(params.Repo),
				"prNum":  githubv4.Int(params.PullNumber),
			}

			if err := client.Query(context.Background(), &getLatestReviewForViewerQuery, vars); err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx,
					"failed to get latest review for current user",
					err,
				), nil
			}

			// Validate there is one review and the state is pending
			if len(getLatestReviewForViewerQuery.Repository.PullRequest.Reviews.Nodes) == 0 {
				return mcp.NewToolResultError("No pending review found for the viewer"), nil
			}

			review := getLatestReviewForViewerQuery.Repository.PullRequest.Reviews.Nodes[0]
			if review.State != githubv4.PullRequestReviewStatePending {
				errText := fmt.Sprintf("The latest review, found at %s is not pending", review.URL)
				return mcp.NewToolResultError(errText), nil
			}

			// Then we can create a new review thread comment on the review.
			var addPullRequestReviewThreadMutation struct {
				AddPullRequestReviewThread struct {
					Thread struct {
						ID githubv4.ID // We don't need this, but a selector is required or GQL complains.
					}
				} `graphql:"addPullRequestReviewThread(input: $input)"`
			}

			if err := client.Mutate(
				ctx,
				&addPullRequestReviewThreadMutation,
				githubv4.AddPullRequestReviewThreadInput{
					Path:                githubv4.String(params.Path),
					Body:                githubv4.String(params.Body),
					SubjectType:         newGQLStringlikePtr[githubv4.PullRequestReviewThreadSubjectType](&params.SubjectType),
					Line:                newGQLIntPtr(params.Line),
					Side:                newGQLStringlikePtr[githubv4.DiffSide](params.Side),
					StartLine:           newGQLIntPtr(params.StartLine),
					StartSide:           newGQLStringlikePtr[githubv4.DiffSide](params.StartSide),
					PullRequestReviewID: &review.ID,
				},
				nil,
			); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Return nothing interesting, just indicate success for the time being.
			// In future, we may want to return the review ID, but for the moment, we're not leaking
			// API implementation details to the LLM.
			return mcp.NewToolResultText("pull request review comment successfully added to pending review"), nil
		}
}



func GetPullRequestDiff(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("get_pull_request_diff",
			mcp.WithDescription(t("TOOL_GET_PULL_REQUEST_DIFF_DESCRIPTION", "Get the diff of a pull request.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_PULL_REQUEST_DIFF_USER_TITLE", "Get pull request diff"),
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
			mcp.WithNumber("pullNumber",
				mcp.Required(),
				mcp.Description("Pull request number"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			var params struct {
				Owner      string
				Repo       string
				PullNumber int32
			}
			if err := mapstructure.Decode(request.Params.Arguments, &params); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to get GitHub client: %v", err)), nil
			}

			raw, resp, err := client.PullRequests.GetRaw(
				ctx,
				params.Owner,
				params.Repo,
				int(params.PullNumber),
				github.RawOptions{Type: github.Diff},
			)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to get pull request diff",
					resp,
					err,
				), nil
			}

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to get pull request diff: %s", string(body))), nil
			}

			defer func() { _ = resp.Body.Close() }()

			// Return the raw response
			return mcp.NewToolResultText(string(raw)), nil
		}
}

// RequestCopilotReview creates a tool to request a Copilot review for a pull request.
// Note that this tool will not work on GHES where this feature is unsupported. In future, we should not expose this
// tool if the configured host does not support it.
func RequestCopilotReview(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("request_copilot_review",
			mcp.WithDescription(t("TOOL_REQUEST_COPILOT_REVIEW_DESCRIPTION", "Request a GitHub Copilot code review for a pull request. Use this for automated feedback on pull requests, usually before requesting a human reviewer.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_REQUEST_COPILOT_REVIEW_USER_TITLE", "Request Copilot review"),
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
			mcp.WithNumber("pullNumber",
				mcp.Required(),
				mcp.Description("Pull request number"),
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

			pullNumber, err := RequiredInt(request, "pullNumber")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			_, resp, err := client.PullRequests.RequestReviewers(
				ctx,
				owner,
				repo,
				pullNumber,
				github.ReviewersRequest{
					// The login name of the copilot reviewer bot
					Reviewers: []string{"copilot-pull-request-reviewer[bot]"},
				},
			)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to request copilot review",
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusCreated {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to request copilot review: %s", string(body))), nil
			}

			// Return nothing on success, as there's not much value in returning the Pull Request itself
			return mcp.NewToolResultText(""), nil
		}
}

func newGQLStringlikePtr[T ~string](s *string) *T {
	if s == nil {
		return nil
	}
	stringlike := T(*s)
	return &stringlike
}

func newGQLIntPtr(i *int32) *githubv4.Int {
	if i == nil {
		return nil
	}
	gi := githubv4.Int(*i)
	return &gi
}
