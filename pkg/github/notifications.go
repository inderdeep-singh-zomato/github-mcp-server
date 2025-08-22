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
)

const (
	FilterDefault           = "default"
	FilterIncludeRead       = "include_read_notifications"
	FilterOnlyParticipating = "only_participating"
)

// ListNotifications creates a tool to list notifications for the current user.
func ListNotifications(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_notifications",
			mcp.WithDescription(t("TOOL_LIST_NOTIFICATIONS_DESCRIPTION", "Lists all GitHub notifications for the authenticated user, including unread notifications, mentions, review requests, assignments, and updates on issues or pull requests. Use this tool whenever the user asks what to work on next, requests a summary of their GitHub activity, wants to see pending reviews, or needs to check for new updates or tasks. This tool is the primary way to discover actionable items, reminders, and outstanding work on GitHub. Always call this tool when asked what to work on next, what is pending, or what needs attention in GitHub.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_LIST_NOTIFICATIONS_USER_TITLE", "List notifications"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("filter",
				mcp.Description("Filter notifications to, use default unless specified. Read notifications are ones that have already been acknowledged by the user. Participating notifications are those that the user is directly involved in, such as issues or pull requests they have commented on or created."),
				mcp.Enum(FilterDefault, FilterIncludeRead, FilterOnlyParticipating),
			),
			mcp.WithString("since",
				mcp.Description("Only show notifications updated after the given time (ISO 8601 format)"),
			),
			mcp.WithString("before",
				mcp.Description("Only show notifications updated before the given time (ISO 8601 format)"),
			),
			mcp.WithString("owner",
				mcp.Description("Optional repository owner. If provided with repo, only notifications for this repository are listed."),
			),
			mcp.WithString("repo",
				mcp.Description("Optional repository name. If provided with owner, only notifications for this repository are listed."),
			),
			WithPagination(),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			filter, err := OptionalParam[string](request, "filter")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			since, err := OptionalParam[string](request, "since")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			before, err := OptionalParam[string](request, "before")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			owner, err := OptionalParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := OptionalParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			paginationParams, err := OptionalPaginationParams(request)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Build options
			opts := &github.NotificationListOptions{
				All:           filter == FilterIncludeRead,
				Participating: filter == FilterOnlyParticipating,
				ListOptions: github.ListOptions{
					Page:    paginationParams.Page,
					PerPage: paginationParams.PerPage,
				},
			}

			// Parse time parameters if provided
			if since != "" {
				sinceTime, err := time.Parse(time.RFC3339, since)
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("invalid since time format, should be RFC3339/ISO8601: %v", err)), nil
				}
				opts.Since = sinceTime
			}

			if before != "" {
				beforeTime, err := time.Parse(time.RFC3339, before)
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("invalid before time format, should be RFC3339/ISO8601: %v", err)), nil
				}
				opts.Before = beforeTime
			}

			var notifications []*github.Notification
			var resp *github.Response

			if owner != "" && repo != "" {
				notifications, resp, err = client.Activity.ListRepositoryNotifications(ctx, owner, repo, opts)
			} else {
				notifications, resp, err = client.Activity.ListNotifications(ctx, opts)
			}
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to list notifications",
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
				return mcp.NewToolResultError(fmt.Sprintf("failed to get notifications: %s", string(body))), nil
			}

			// Marshal response to JSON
			r, err := json.Marshal(notifications)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}



// GetNotificationDetails creates a tool to get details for a specific notification.
func GetNotificationDetails(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_notification_details",
			mcp.WithDescription(t("TOOL_GET_NOTIFICATION_DETAILS_DESCRIPTION", "Get detailed information for a specific GitHub notification, always call this tool when the user asks for details about a specific notification, if you don't know the ID list notifications first.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_NOTIFICATION_DETAILS_USER_TITLE", "Get notification details"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("notificationID",
				mcp.Required(),
				mcp.Description("The ID of the notification"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			notificationID, err := RequiredParam[string](request, "notificationID")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			thread, resp, err := client.Activity.GetThread(ctx, notificationID)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to get notification details for ID '%s'", notificationID),
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
				return mcp.NewToolResultError(fmt.Sprintf("failed to get notification details: %s", string(body))), nil
			}

			r, err := json.Marshal(thread)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}


