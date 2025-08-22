package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v74/github"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ListNotifications(t *testing.T) {
	// Verify tool definition and schema
	mockClient := github.NewClient(nil)
	tool, _ := ListNotifications(stubGetClientFn(mockClient), translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "list_notifications", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "filter")
	assert.Contains(t, tool.InputSchema.Properties, "since")
	assert.Contains(t, tool.InputSchema.Properties, "before")
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "page")
	assert.Contains(t, tool.InputSchema.Properties, "perPage")
	// All fields are optional, so Required should be empty
	assert.Empty(t, tool.InputSchema.Required)

	mockNotification := &github.Notification{
		ID:     github.Ptr("123"),
		Reason: github.Ptr("mention"),
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedResult []*github.Notification
		expectedErrMsg string
	}{
		{
			name: "success default filter (no params)",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetNotifications,
					[]*github.Notification{mockNotification},
				),
			),
			requestArgs:    map[string]interface{}{},
			expectError:    false,
			expectedResult: []*github.Notification{mockNotification},
		},
		{
			name: "success with filter=include_read_notifications",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetNotifications,
					[]*github.Notification{mockNotification},
				),
			),
			requestArgs: map[string]interface{}{
				"filter": "include_read_notifications",
			},
			expectError:    false,
			expectedResult: []*github.Notification{mockNotification},
		},
		{
			name: "success with filter=only_participating",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetNotifications,
					[]*github.Notification{mockNotification},
				),
			),
			requestArgs: map[string]interface{}{
				"filter": "only_participating",
			},
			expectError:    false,
			expectedResult: []*github.Notification{mockNotification},
		},
		{
			name: "success for repo notifications",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetReposNotificationsByOwnerByRepo,
					[]*github.Notification{mockNotification},
				),
			),
			requestArgs: map[string]interface{}{
				"filter":  "default",
				"since":   "2024-01-01T00:00:00Z",
				"before":  "2024-01-02T00:00:00Z",
				"owner":   "octocat",
				"repo":    "hello-world",
				"page":    float64(2),
				"perPage": float64(10),
			},
			expectError:    false,
			expectedResult: []*github.Notification{mockNotification},
		},
		{
			name: "error",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetNotifications,
					mockResponse(t, http.StatusInternalServerError, `{"message": "error"}`),
				),
			),
			requestArgs:    map[string]interface{}{},
			expectError:    true,
			expectedErrMsg: "error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := github.NewClient(tc.mockedClient)
			_, handler := ListNotifications(stubGetClientFn(client), translations.NullTranslationHelper)
			request := createMCPRequest(tc.requestArgs)
			result, err := handler(context.Background(), request)

			if tc.expectError {
				require.NoError(t, err)
				require.True(t, result.IsError)
				errorContent := getErrorResult(t, result)
				if tc.expectedErrMsg != "" {
					assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				}
				return
			}

			require.NoError(t, err)
			require.False(t, result.IsError)
			textContent := getTextResult(t, result)
			t.Logf("textContent: %s", textContent.Text)
			var returned []*github.Notification
			err = json.Unmarshal([]byte(textContent.Text), &returned)
			require.NoError(t, err)
			require.NotEmpty(t, returned)
			assert.Equal(t, *tc.expectedResult[0].ID, *returned[0].ID)
		})
	}
}





func Test_GetNotificationDetails(t *testing.T) {
	// Verify tool definition and schema
	mockClient := github.NewClient(nil)
	tool, _ := GetNotificationDetails(stubGetClientFn(mockClient), translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "get_notification_details", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "notificationID")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"notificationID"})

	mockThread := &github.Notification{ID: github.Ptr("123"), Reason: github.Ptr("mention")}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectResult   *github.Notification
		expectedErrMsg string
	}{
		{
			name: "success",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetNotificationsThreadsByThreadId,
					mockThread,
				),
			),
			requestArgs: map[string]interface{}{
				"notificationID": "123",
			},
			expectError:  false,
			expectResult: mockThread,
		},
		{
			name: "not found",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetNotificationsThreadsByThreadId,
					mockResponse(t, http.StatusNotFound, `{"message": "not found"}`),
				),
			),
			requestArgs: map[string]interface{}{
				"notificationID": "123",
			},
			expectError:    true,
			expectedErrMsg: "not found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := github.NewClient(tc.mockedClient)
			_, handler := GetNotificationDetails(stubGetClientFn(client), translations.NullTranslationHelper)
			request := createMCPRequest(tc.requestArgs)
			result, err := handler(context.Background(), request)

			if tc.expectError {
				require.NoError(t, err)
				require.True(t, result.IsError)
				errorContent := getErrorResult(t, result)
				if tc.expectedErrMsg != "" {
					assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				}
				return
			}

			require.NoError(t, err)
			require.False(t, result.IsError)
			textContent := getTextResult(t, result)
			var returned github.Notification
			err = json.Unmarshal([]byte(textContent.Text), &returned)
			require.NoError(t, err)
			assert.Equal(t, *tc.expectResult.ID, *returned.ID)
		})
	}
}
