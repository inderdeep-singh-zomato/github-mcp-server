package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v74/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/shurcooL/githubv4"
)

// IssueFragment represents a fragment of an issue node in the GraphQL API.
type IssueFragment struct {
	Number     githubv4.Int
	Title      githubv4.String
	Body       githubv4.String
	State      githubv4.String
	DatabaseID int64

	Author struct {
		Login githubv4.String
	}
	CreatedAt githubv4.DateTime
	UpdatedAt githubv4.DateTime
	Labels    struct {
		Nodes []struct {
			Name        githubv4.String
			ID          githubv4.String
			Description githubv4.String
		}
	} `graphql:"labels(first: 100)"`
	Comments struct {
		TotalCount githubv4.Int
	} `graphql:"comments"`
}

// Common interface for all issue query types
type IssueQueryResult interface {
	GetIssueFragment() IssueQueryFragment
}

type IssueQueryFragment struct {
	Nodes    []IssueFragment `graphql:"nodes"`
	PageInfo struct {
		HasNextPage     githubv4.Boolean
		HasPreviousPage githubv4.Boolean
		StartCursor     githubv4.String
		EndCursor       githubv4.String
	}
	TotalCount int
}

// ListIssuesQuery is the root query structure for fetching issues with optional label filtering.
type ListIssuesQuery struct {
	Repository struct {
		Issues IssueQueryFragment `graphql:"issues(first: $first, after: $after, states: $states, orderBy: {field: $orderBy, direction: $direction})"`
	} `graphql:"repository(owner: $owner, name: $repo)"`
}

// ListIssuesQueryTypeWithLabels is the query structure for fetching issues with optional label filtering.
type ListIssuesQueryTypeWithLabels struct {
	Repository struct {
		Issues IssueQueryFragment `graphql:"issues(first: $first, after: $after, labels: $labels, states: $states, orderBy: {field: $orderBy, direction: $direction})"`
	} `graphql:"repository(owner: $owner, name: $repo)"`
}

// ListIssuesQueryWithSince is the query structure for fetching issues without label filtering but with since filtering.
type ListIssuesQueryWithSince struct {
	Repository struct {
		Issues IssueQueryFragment `graphql:"issues(first: $first, after: $after, states: $states, orderBy: {field: $orderBy, direction: $direction}, filterBy: {since: $since})"`
	} `graphql:"repository(owner: $owner, name: $repo)"`
}

// ListIssuesQueryTypeWithLabelsWithSince is the query structure for fetching issues with both label and since filtering.
type ListIssuesQueryTypeWithLabelsWithSince struct {
	Repository struct {
		Issues IssueQueryFragment `graphql:"issues(first: $first, after: $after, labels: $labels, states: $states, orderBy: {field: $orderBy, direction: $direction}, filterBy: {since: $since})"`
	} `graphql:"repository(owner: $owner, name: $repo)"`
}

// Implement the interface for all query types
func (q *ListIssuesQueryTypeWithLabels) GetIssueFragment() IssueQueryFragment {
	return q.Repository.Issues
}

func (q *ListIssuesQuery) GetIssueFragment() IssueQueryFragment {
	return q.Repository.Issues
}

func (q *ListIssuesQueryWithSince) GetIssueFragment() IssueQueryFragment {
	return q.Repository.Issues
}

func (q *ListIssuesQueryTypeWithLabelsWithSince) GetIssueFragment() IssueQueryFragment {
	return q.Repository.Issues
}

func getIssueQueryType(hasLabels bool, hasSince bool) any {
	switch {
	case hasLabels && hasSince:
		return &ListIssuesQueryTypeWithLabelsWithSince{}
	case hasLabels:
		return &ListIssuesQueryTypeWithLabels{}
	case hasSince:
		return &ListIssuesQueryWithSince{}
	default:
		return &ListIssuesQuery{}
	}
}

func fragmentToIssue(fragment IssueFragment) *github.Issue {
	// Convert GraphQL labels to GitHub API labels format
	var foundLabels []*github.Label
	for _, labelNode := range fragment.Labels.Nodes {
		foundLabels = append(foundLabels, &github.Label{
			Name:        github.Ptr(string(labelNode.Name)),
			NodeID:      github.Ptr(string(labelNode.ID)),
			Description: github.Ptr(string(labelNode.Description)),
		})
	}

	return &github.Issue{
		Number:    github.Ptr(int(fragment.Number)),
		Title:     github.Ptr(string(fragment.Title)),
		CreatedAt: &github.Timestamp{Time: fragment.CreatedAt.Time},
		UpdatedAt: &github.Timestamp{Time: fragment.UpdatedAt.Time},
		User: &github.User{
			Login: github.Ptr(string(fragment.Author.Login)),
		},
		State:    github.Ptr(string(fragment.State)),
		ID:       github.Ptr(fragment.DatabaseID),
		Body:     github.Ptr(string(fragment.Body)),
		Labels:   foundLabels,
		Comments: github.Ptr(int(fragment.Comments.TotalCount)),
	}
}

// GetIssue creates a tool to get details of a specific issue in a GitHub repository.
func GetIssue(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_issue",
			mcp.WithDescription(t("TOOL_GET_ISSUE_DESCRIPTION", "Get details of a specific issue in a GitHub repository.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_ISSUE_USER_TITLE", "Get issue details"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("The owner of the repository"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("The name of the repository"),
			),
			mcp.WithNumber("issue_number",
				mcp.Required(),
				mcp.Description("The number of the issue"),
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
			issueNumber, err := RequiredInt(request, "issue_number")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			issue, resp, err := client.Issues.Get(ctx, owner, repo, issueNumber)
			if err != nil {
				return nil, fmt.Errorf("failed to get issue: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to get issue: %s", string(body))), nil
			}

			r, err := json.Marshal(issue)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal issue: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// ListIssueTypes creates a tool to list defined issue types for an organization. This can be used to understand supported issue type values for creating or updating issues.
func ListIssueTypes(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {

	return mcp.NewTool("list_issue_types",
			mcp.WithDescription(t("TOOL_LIST_ISSUE_TYPES_FOR_ORG", "List supported issue types for repository owner (organization).")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_LIST_ISSUE_TYPES_USER_TITLE", "List available issue types"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("The organization owner of the repository"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			issueTypes, resp, err := client.Organizations.ListIssueTypes(ctx, owner)
			if err != nil {
				return nil, fmt.Errorf("failed to list issue types: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to list issue types: %s", string(body))), nil
			}

			r, err := json.Marshal(issueTypes)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal issue types: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// AddIssueComment creates a tool to add a comment to an issue.
func AddIssueComment(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("add_issue_comment",
			mcp.WithDescription(t("TOOL_ADD_ISSUE_COMMENT_DESCRIPTION", "Add a comment to a specific issue in a GitHub repository.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_ADD_ISSUE_COMMENT_USER_TITLE", "Add comment to issue"),
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
			mcp.WithNumber("issue_number",
				mcp.Required(),
				mcp.Description("Issue number to comment on"),
			),
			mcp.WithString("body",
				mcp.Required(),
				mcp.Description("Comment content"),
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
			issueNumber, err := RequiredInt(request, "issue_number")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			body, err := RequiredParam[string](request, "body")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			comment := &github.IssueComment{
				Body: github.Ptr(body),
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			createdComment, resp, err := client.Issues.CreateComment(ctx, owner, repo, issueNumber, comment)
			if err != nil {
				return nil, fmt.Errorf("failed to create comment: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusCreated {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to create comment: %s", string(body))), nil
			}

			r, err := json.Marshal(createdComment)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// AddSubIssue creates a tool to add a sub-issue to a parent issue.
func AddSubIssue(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("add_sub_issue",
			mcp.WithDescription(t("TOOL_ADD_SUB_ISSUE_DESCRIPTION", "Add a sub-issue to a parent issue in a GitHub repository.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_ADD_SUB_ISSUE_USER_TITLE", "Add sub-issue"),
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
			mcp.WithNumber("issue_number",
				mcp.Required(),
				mcp.Description("The number of the parent issue"),
			),
			mcp.WithNumber("sub_issue_id",
				mcp.Required(),
				mcp.Description("The ID of the sub-issue to add. ID is not the same as issue number"),
			),
			mcp.WithBoolean("replace_parent",
				mcp.Description("When true, replaces the sub-issue's current parent issue"),
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
			issueNumber, err := RequiredInt(request, "issue_number")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			subIssueID, err := RequiredInt(request, "sub_issue_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			replaceParent, err := OptionalParam[bool](request, "replace_parent")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			subIssueRequest := github.SubIssueRequest{
				SubIssueID:    int64(subIssueID),
				ReplaceParent: ToBoolPtr(replaceParent),
			}

			subIssue, resp, err := client.SubIssue.Add(ctx, owner, repo, int64(issueNumber), subIssueRequest)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to add sub-issue",
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
				return mcp.NewToolResultError(fmt.Sprintf("failed to add sub-issue: %s", string(body))), nil
			}

			r, err := json.Marshal(subIssue)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// ListSubIssues creates a tool to list sub-issues for a GitHub issue.
func ListSubIssues(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_sub_issues",
			mcp.WithDescription(t("TOOL_LIST_SUB_ISSUES_DESCRIPTION", "List sub-issues for a specific issue in a GitHub repository.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_LIST_SUB_ISSUES_USER_TITLE", "List sub-issues"),
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
			mcp.WithNumber("issue_number",
				mcp.Required(),
				mcp.Description("Issue number"),
			),
			mcp.WithNumber("page",
				mcp.Description("Page number for pagination (default: 1)"),
			),
			mcp.WithNumber("per_page",
				mcp.Description("Number of results per page (max 100, default: 30)"),
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
			issueNumber, err := RequiredInt(request, "issue_number")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			page, err := OptionalIntParamWithDefault(request, "page", 1)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			perPage, err := OptionalIntParamWithDefault(request, "per_page", 30)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			opts := &github.IssueListOptions{
				ListOptions: github.ListOptions{
					Page:    page,
					PerPage: perPage,
				},
			}

			subIssues, resp, err := client.SubIssue.ListByIssue(ctx, owner, repo, int64(issueNumber), opts)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to list sub-issues",
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
				return mcp.NewToolResultError(fmt.Sprintf("failed to list sub-issues: %s", string(body))), nil
			}

			r, err := json.Marshal(subIssues)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}

}



// SearchIssues creates a tool to search for issues.
func SearchIssues(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("search_issues",
			mcp.WithDescription(t("TOOL_SEARCH_ISSUES_DESCRIPTION", "Search for issues in GitHub repositories using issues search syntax already scoped to is:issue")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_SEARCH_ISSUES_USER_TITLE", "Search issues"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("Search query using GitHub issues search syntax"),
			),
			mcp.WithString("owner",
				mcp.Description("Optional repository owner. If provided with repo, only issues for this repository are listed."),
			),
			mcp.WithString("repo",
				mcp.Description("Optional repository name. If provided with owner, only issues for this repository are listed."),
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
			return searchHandler(ctx, getClient, request, "issue", "failed to search issues")
		}
}

// CreateIssue creates a tool to create a new issue in a GitHub repository.
func CreateIssue(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("create_issue",
			mcp.WithDescription(t("TOOL_CREATE_ISSUE_DESCRIPTION", "Create a new issue in a GitHub repository.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_CREATE_ISSUE_USER_TITLE", "Open new issue"),
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
				mcp.Description("Issue title"),
			),
			mcp.WithString("body",
				mcp.Description("Issue body content"),
			),
			mcp.WithArray("assignees",
				mcp.Description("Usernames to assign to this issue"),
				mcp.Items(
					map[string]any{
						"type": "string",
					},
				),
			),
			mcp.WithArray("labels",
				mcp.Description("Labels to apply to this issue"),
				mcp.Items(
					map[string]any{
						"type": "string",
					},
				),
			),
			mcp.WithNumber("milestone",
				mcp.Description("Milestone number"),
			),
			mcp.WithString("type",
				mcp.Description("Type of this issue"),
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

			// Optional parameters
			body, err := OptionalParam[string](request, "body")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Get assignees
			assignees, err := OptionalStringArrayParam(request, "assignees")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Get labels
			labels, err := OptionalStringArrayParam(request, "labels")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Get optional milestone
			milestone, err := OptionalIntParam(request, "milestone")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			var milestoneNum *int
			if milestone != 0 {
				milestoneNum = &milestone
			}

			// Get optional type
			issueType, err := OptionalParam[string](request, "type")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Create the issue request
			issueRequest := &github.IssueRequest{
				Title:     github.Ptr(title),
				Body:      github.Ptr(body),
				Assignees: &assignees,
				Labels:    &labels,
				Milestone: milestoneNum,
			}

			if issueType != "" {
				issueRequest.Type = github.Ptr(issueType)
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			issue, resp, err := client.Issues.Create(ctx, owner, repo, issueRequest)
			if err != nil {
				return nil, fmt.Errorf("failed to create issue: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusCreated {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to create issue: %s", string(body))), nil
			}

			r, err := json.Marshal(issue)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// ListIssues creates a tool to list and filter repository issues
func ListIssues(getGQLClient GetGQLClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_issues",
			mcp.WithDescription(t("TOOL_LIST_ISSUES_DESCRIPTION", "List issues in a GitHub repository. For pagination, use the 'endCursor' from the previous response's 'pageInfo' in the 'after' parameter.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_LIST_ISSUES_USER_TITLE", "List issues"),
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
				mcp.Description("Filter by state, by default both open and closed issues are returned when not provided"),
				mcp.Enum("OPEN", "CLOSED"),
			),
			mcp.WithArray("labels",
				mcp.Description("Filter by labels"),
				mcp.Items(
					map[string]interface{}{
						"type": "string",
					},
				),
			),
			mcp.WithString("orderBy",
				mcp.Description("Order issues by field. If provided, the 'direction' also needs to be provided."),
				mcp.Enum("CREATED_AT", "UPDATED_AT", "COMMENTS"),
			),
			mcp.WithString("direction",
				mcp.Description("Order direction. If provided, the 'orderBy' also needs to be provided."),
				mcp.Enum("ASC", "DESC"),
			),
			mcp.WithString("since",
				mcp.Description("Filter by date (ISO 8601 timestamp)"),
			),
			WithCursorPagination(),
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

			// Set optional parameters if provided
			state, err := OptionalParam[string](request, "state")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// If the state has a value, cast into an array of strings
			var states []githubv4.IssueState
			if state != "" {
				states = append(states, githubv4.IssueState(state))
			} else {
				states = []githubv4.IssueState{githubv4.IssueStateOpen, githubv4.IssueStateClosed}
			}

			// Get labels
			labels, err := OptionalStringArrayParam(request, "labels")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			orderBy, err := OptionalParam[string](request, "orderBy")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			direction, err := OptionalParam[string](request, "direction")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// These variables are required for the GraphQL query to be set by default
			// If orderBy is empty, default to CREATED_AT
			if orderBy == "" {
				orderBy = "CREATED_AT"
			}
			// If direction is empty, default to DESC
			if direction == "" {
				direction = "DESC"
			}

			since, err := OptionalParam[string](request, "since")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// There are two optional parameters: since and labels.
			var sinceTime time.Time
			var hasSince bool
			if since != "" {
				sinceTime, err = parseISOTimestamp(since)
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("failed to list issues: %s", err.Error())), nil
				}
				hasSince = true
			}
			hasLabels := len(labels) > 0

			// Get pagination parameters and convert to GraphQL format
			pagination, err := OptionalCursorPaginationParams(request)
			if err != nil {
				return nil, err
			}

			// Check if someone tried to use page-based pagination instead of cursor-based
			if _, pageProvided := request.GetArguments()["page"]; pageProvided {
				return mcp.NewToolResultError("This tool uses cursor-based pagination. Use the 'after' parameter with the 'endCursor' value from the previous response instead of 'page'."), nil
			}

			// Check if pagination parameters were explicitly provided
			_, perPageProvided := request.GetArguments()["perPage"]
			paginationExplicit := perPageProvided

			paginationParams, err := pagination.ToGraphQLParams()
			if err != nil {
				return nil, err
			}

			// Use default of 30 if pagination was not explicitly provided
			if !paginationExplicit {
				defaultFirst := int32(DefaultGraphQLPageSize)
				paginationParams.First = &defaultFirst
			}

			client, err := getGQLClient(ctx)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to get GitHub GQL client: %v", err)), nil
			}

			vars := map[string]interface{}{
				"owner":     githubv4.String(owner),
				"repo":      githubv4.String(repo),
				"states":    states,
				"orderBy":   githubv4.IssueOrderField(orderBy),
				"direction": githubv4.OrderDirection(direction),
				"first":     githubv4.Int(*paginationParams.First),
			}

			if paginationParams.After != nil {
				vars["after"] = githubv4.String(*paginationParams.After)
			} else {
				// Used within query, therefore must be set to nil and provided as $after
				vars["after"] = (*githubv4.String)(nil)
			}

			// Ensure optional parameters are set
			if hasLabels {
				// Use query with labels filtering - convert string labels to githubv4.String slice
				labelStrings := make([]githubv4.String, len(labels))
				for i, label := range labels {
					labelStrings[i] = githubv4.String(label)
				}
				vars["labels"] = labelStrings
			}

			if hasSince {
				vars["since"] = githubv4.DateTime{Time: sinceTime}
			}

			issueQuery := getIssueQueryType(hasLabels, hasSince)
			if err := client.Query(ctx, issueQuery, vars); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Extract and convert all issue nodes using the common interface
			var issues []*github.Issue
			var pageInfo struct {
				HasNextPage     githubv4.Boolean
				HasPreviousPage githubv4.Boolean
				StartCursor     githubv4.String
				EndCursor       githubv4.String
			}
			var totalCount int

			if queryResult, ok := issueQuery.(IssueQueryResult); ok {
				fragment := queryResult.GetIssueFragment()
				for _, issue := range fragment.Nodes {
					issues = append(issues, fragmentToIssue(issue))
				}
				pageInfo = fragment.PageInfo
				totalCount = fragment.TotalCount
			}

			// Create response with issues
			response := map[string]interface{}{
				"issues": issues,
				"pageInfo": map[string]interface{}{
					"hasNextPage":     pageInfo.HasNextPage,
					"hasPreviousPage": pageInfo.HasPreviousPage,
					"startCursor":     string(pageInfo.StartCursor),
					"endCursor":       string(pageInfo.EndCursor),
				},
				"totalCount": totalCount,
			}
			out, err := json.Marshal(response)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal issues: %w", err)
			}
			return mcp.NewToolResultText(string(out)), nil
		}
}


// GetIssueComments creates a tool to get comments for a GitHub issue.
func GetIssueComments(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_issue_comments",
			mcp.WithDescription(t("TOOL_GET_ISSUE_COMMENTS_DESCRIPTION", "Get comments for a specific issue in a GitHub repository.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_ISSUE_COMMENTS_USER_TITLE", "Get issue comments"),
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
			mcp.WithNumber("issue_number",
				mcp.Required(),
				mcp.Description("Issue number"),
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
			issueNumber, err := RequiredInt(request, "issue_number")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			pagination, err := OptionalPaginationParams(request)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			opts := &github.IssueListCommentsOptions{
				ListOptions: github.ListOptions{
					Page:    pagination.Page,
					PerPage: pagination.PerPage,
				},
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			comments, resp, err := client.Issues.ListComments(ctx, owner, repo, issueNumber, opts)
			if err != nil {
				return nil, fmt.Errorf("failed to get issue comments: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to get issue comments: %s", string(body))), nil
			}

			r, err := json.Marshal(comments)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}


// parseISOTimestamp parses an ISO 8601 timestamp string into a time.Time object.
// Returns the parsed time or an error if parsing fails.
// Example formats supported: "2023-01-15T14:30:00Z", "2023-01-15"
func parseISOTimestamp(timestamp string) (time.Time, error) {
	if timestamp == "" {
		return time.Time{}, fmt.Errorf("empty timestamp")
	}

	// Try RFC3339 format (standard ISO 8601 with time)
	t, err := time.Parse(time.RFC3339, timestamp)
	if err == nil {
		return t, nil
	}

	// Try simple date format (YYYY-MM-DD)
	t, err = time.Parse("2006-01-02", timestamp)
	if err == nil {
		return t, nil
	}

	// Return error with supported formats
	return time.Time{}, fmt.Errorf("invalid ISO 8601 timestamp: %s (supported formats: YYYY-MM-DDThh:mm:ssZ or YYYY-MM-DD)", timestamp)
}

func AssignCodingAgentPrompt(t translations.TranslationHelperFunc) (tool mcp.Prompt, handler server.PromptHandlerFunc) {
	return mcp.NewPrompt("AssignCodingAgent",
			mcp.WithPromptDescription(t("PROMPT_ASSIGN_CODING_AGENT_DESCRIPTION", "Assign GitHub Coding Agent to multiple tasks in a GitHub repository.")),
			mcp.WithArgument("repo", mcp.ArgumentDescription("The repository to assign tasks in (owner/repo)."), mcp.RequiredArgument()),
		), func(_ context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			repo := request.Params.Arguments["repo"]

			messages := []mcp.PromptMessage{
				{
					Role:    "user",
					Content: mcp.NewTextContent("You are a personal assistant for GitHub the Copilot GitHub Coding Agent. Your task is to help the user assign tasks to the Coding Agent based on their open GitHub issues. You can use `assign_copilot_to_issue` tool to assign the Coding Agent to issues that are suitable for autonomous work, and `search_issues` tool to find issues that match the user's criteria. You can also use `list_issues` to get a list of issues in the repository."),
				},
				{
					Role:    "user",
					Content: mcp.NewTextContent(fmt.Sprintf("Please go and get a list of the most recent 10 issues from the %s GitHub repository", repo)),
				},
				{
					Role:    "assistant",
					Content: mcp.NewTextContent(fmt.Sprintf("Sure! I will get a list of the 10 most recent issues for the repo %s.", repo)),
				},
				{
					Role:    "user",
					Content: mcp.NewTextContent("For each issue, please check if it is a clearly defined coding task with acceptance criteria and a low to medium complexity to identify issues that are suitable for an AI Coding Agent to work on. Then assign each of the identified issues to Copilot."),
				},
				{
					Role:    "assistant",
					Content: mcp.NewTextContent("Certainly! Let me carefully check which ones are clearly scoped issues that are good to assign to the coding agent, and I will summarize and assign them now."),
				},
				{
					Role:    "user",
					Content: mcp.NewTextContent("Great, if you are unsure if an issue is good to assign, ask me first, rather than assigning copilot. If you are certain the issue is clear and suitable you can assign it to Copilot without asking."),
				},
			}
			return &mcp.GetPromptResult{
				Messages: messages,
			}, nil
		}
}
