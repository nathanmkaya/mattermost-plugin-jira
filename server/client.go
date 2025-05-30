// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"

	jira "github.com/andygrunwald/go-jira"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/pluginapi"

	"github.com/mattermost/mattermost-plugin-jira/server/utils/types"
)

const autocompleteSearchRoute = "2/jql/autocompletedata/suggestions"
const commentVisibilityRoute = "2/user"
const userSearchRoute = "2/user/assignable/search"
const unrecognizedEndpoint = "_unrecognized"
const visibleToAllUsers = "visible-to-all-users"

// Client is the combined interface for all upstream APIs and convenience methods.
type Client interface {
	RESTService
	IssueService
	ProjectService
	SearchService
	UserService
}

// RESTService is the low-level interface for invoking the upstream service.
// endoint can be a "short" API URL path, including the version desired, like "3/user",
// or a fully-qualified URL, with a non-empty Scheme.
type RESTService interface {
	RESTGet(endpoint string, params map[string]string, dest interface{}) error
	RESTPostAttachment(issueID string, data io.Reader, name string) (*jira.Attachment, error)
}

// UserService is the interface for user-related APIs.
type UserService interface {
	GetSelf() (*jira.User, error)
	GetUserGroups(connection *Connection) ([]*jira.UserGroup, error)
}

// ProjectService is the interface for project-related APIs.
type ProjectService interface {
	GetProject(key string) (*jira.Project, error)
	ListProjects(query string, limit int, expandIssueTypes bool) (jira.ProjectList, error)
	GetIssueTypes(projectID string) ([]jira.IssueType, error)
	ListProjectStatuses(projectID string) ([]*IssueTypeWithStatuses, error)
}

// SearchService is the interface for search-related APIs.
type SearchService interface {
	SearchIssues(jql string, options *jira.SearchOptions) ([]jira.Issue, error)
	SearchUsersAssignableToIssue(issueKey, query string, maxResults int) ([]jira.User, error)
	SearchUsersAssignableInProject(projectKey, query string, maxResults int) ([]jira.User, error)
	SearchAutoCompleteFields(params map[string]string) (*AutoCompleteResult, error)
	GetUserVisibilityGroups(params map[string]string) (*CommentVisibilityResult, error)
}

// IssueService is the interface for issue-related APIs.
type IssueService interface {
	GetIssue(key string, options *jira.GetQueryOptions) (*jira.Issue, error)
	CreateIssue(issue *jira.Issue) (*jira.Issue, error)

	AddAttachment(mmClient pluginapi.Client, issueKey, fileID string, maxSize types.ByteSize) (mattermostName, jiraName, mime string, err error)
	AddComment(issueKey string, comment *jira.Comment) (*jira.Comment, error)
	DoTransition(issueKey, transitionID string) error
	GetCreateMetaInfo(api plugin.API, options *jira.GetQueryOptions) (*jira.CreateMetaInfo, error)
	GetTransitions(issueKey string) ([]jira.Transition, error)
	UpdateAssignee(issueKey string, user *jira.User) error
	UpdateComment(issueKey string, comment *jira.Comment) (*jira.Comment, error)
}

// JiraClient is the common implementation of most Jira APIs, except those that are
// Jira Server or Jira Cloud specific.
type JiraClient struct {
	Jira *jira.Client
}

// RESTGet calls a specified HTTP point with a GET method. endpoint must be an absolute URL, or a
// relative URL starting with a version, like "2/user".
func (client JiraClient) RESTGet(endpoint string, params map[string]string, dest interface{}) error {
	endpointURL, err := endpointURL(endpoint)
	if err != nil {
		return err
	}
	req, err := client.Jira.NewRequest("GET", endpointURL, nil)
	if err != nil {
		return err
	}
	q := req.URL.Query()
	for k, v := range params {
		q.Add(k, v)
	}
	req.URL.RawQuery = q.Encode()

	resp, err := client.Jira.Do(req, dest)
	if err != nil {
		err = userFriendlyJiraError(resp, err)
	}
	return err
}

// RESTPostAttachment uploads an attachment to an issue. The reason for the custom implementation,
// as opposed to using the Issue.PostAttachment() API is that between Jira and the API
// implementation, the error handling is broken.
//
// `issue/%s/attachments` endpoint returns some errors as JSON ("You do not have permission to
// create attachments for this issue"), and some as plain text ("The field file exceeds its maximum
// permitted size of 1024 bytes"). This implementation handles both.
func (client JiraClient) RESTPostAttachment(issueID string, data io.Reader, name string) (*jira.Attachment, error) {
	endpointURL, err := endpointURL(fmt.Sprintf("2/issue/%s/attachments", issueID))
	if err != nil {
		return nil, err
	}

	b := new(bytes.Buffer)
	writer := multipart.NewWriter(b)
	defer writer.Close()
	fw, err := writer.CreateFormFile("file", name)
	if err != nil {
		return nil, err
	}
	if _, err = io.Copy(fw, data); err != nil {
		return nil, err
	}
	writer.Close()
	req, err := client.Jira.NewMultiPartRequest("POST", endpointURL, b)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// PostAttachment response returns a JSON array (as multiple attachments can be posted)
	attachments := []*jira.Attachment{}
	resp, err := client.Jira.Do(req, &attachments)

	if err != nil {
		if resp == nil {
			return nil, err
		}

		bb, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		jerr := jira.Error{}
		jsonerr := json.Unmarshal(bb, &jerr)
		if jsonerr == nil {
			err = userFriendlyJiraError(nil, &jerr)
		} else {
			err = errors.New(" - " + string(bb))
		}
		return nil, RESTError{err, resp.StatusCode}
	}
	if len(attachments) != 1 {
		return nil, errors.Errorf("expected 1 attachment, got %v", len(attachments))
	}

	return attachments[0], nil
}

// GetProject returns a Project by key.
func (client JiraClient) GetProject(key string) (*jira.Project, error) {
	project, resp, err := client.Jira.Project.Get(key)
	if err != nil {
		return nil, userFriendlyJiraError(resp, err)
	}
	return project, nil
}

// GetIssue returns an Issue by key (with options).
func (client JiraClient) GetIssue(key string, options *jira.GetQueryOptions) (*jira.Issue, error) {
	issue, resp, err := client.Jira.Issue.Get(key, options)
	if err != nil {
		return nil, userFriendlyJiraError(resp, err)
	}
	return issue, nil
}

// GetTransitions returns transitions for an issue with issueKey.
func (client JiraClient) GetTransitions(issueKey string) ([]jira.Transition, error) {
	transitions, resp, err := client.Jira.Issue.GetTransitions(issueKey)
	if err != nil {
		return nil, userFriendlyJiraError(resp, err)
	}
	return transitions, err
}

// CreateIssue creates and returns a new issue.
func (client JiraClient) CreateIssue(issue *jira.Issue) (*jira.Issue, error) {
	created, resp, err := client.Jira.Issue.Create(issue)
	if err != nil {
		return nil, userFriendlyJiraError(resp, err)
	}
	return created, nil
}

// UpdateAssignee changes the user assigned to an issue.
func (client JiraClient) UpdateAssignee(issueKey string, user *jira.User) error {
	resp, err := client.Jira.Issue.UpdateAssignee(issueKey, user)
	if err != nil {
		return userFriendlyJiraError(resp, err)
	}
	return err
}

// AddComment adds a comment to an issue.
func (client JiraClient) AddComment(issueKey string, comment *jira.Comment) (*jira.Comment, error) {
	added, resp, err := client.Jira.Issue.AddComment(issueKey, comment)
	if err != nil {
		return nil, userFriendlyJiraError(resp, err)
	}
	return added, err
}

// UpdateComment changes a comment of an issue.
func (client JiraClient) UpdateComment(issueKey string, comment *jira.Comment) (*jira.Comment, error) {
	updated, resp, err := client.Jira.Issue.UpdateComment(issueKey, comment)
	if err != nil {
		return nil, userFriendlyJiraError(resp, err)
	}
	return updated, err
}

// SearchIssues searches issues as specified by jql and options.
func (client JiraClient) SearchIssues(jql string, options *jira.SearchOptions) ([]jira.Issue, error) {
	found, resp, err := client.Jira.Issue.Search(jql, options)
	if err != nil {
		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
			return nil, errors.New("not authorized to search issues")
		}
		return nil, userFriendlyJiraError(resp, err)
	}
	return found, nil
}

type Result struct {
	Value       string `json:"value"`
	DisplayName string `json:"displayName"`
}

type AutoCompleteResult struct {
	Results []Result `json:"results"`
}

type JiraUserGroup struct {
	Name string `json:"name"`
}

type JiraUserGroupCollection struct {
	JiraUserGroups []*JiraUserGroup `json:"items"`
}

type CommentVisibilityResult struct {
	Groups *JiraUserGroupCollection `json:"groups"`
}

// SearchAutoCompleteFields searches fieldValue specified in the params and returns autocomplete suggestions
// for that fieldValue
func (client JiraClient) SearchAutoCompleteFields(params map[string]string) (*AutoCompleteResult, error) {
	result := &AutoCompleteResult{}
	err := client.RESTGet(autocompleteSearchRoute, params, result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// GetUserVisibilityGroups searches fieldValue specified in the params and returns the comment visibility suggestions
// for that fieldValue
func (client JiraClient) GetUserVisibilityGroups(params map[string]string) (*CommentVisibilityResult, error) {
	result := &CommentVisibilityResult{}
	if err := client.RESTGet(commentVisibilityRoute, params, result); err != nil {
		return nil, err
	}
	result.Groups.JiraUserGroups = append(result.Groups.JiraUserGroups, &JiraUserGroup{visibleToAllUsers})
	return result, nil
}

// DoTransition executes a transition on an issue.
func (client JiraClient) DoTransition(issueKey, transitionID string) error {
	resp, err := client.Jira.Issue.DoTransition(issueKey, transitionID)
	if err != nil {
		return userFriendlyJiraError(resp, err)
	}
	return nil
}

// AddAttachment uploads a file attachment
func (client JiraClient) AddAttachment(mmClient pluginapi.Client, issueKey, fileID string, maxSize types.ByteSize) (
	mattermostName, jiraName, mime string, err error) {
	fileinfo, err := mmClient.File.GetInfo(fileID)
	if err != nil {
		return "", "", "", err
	}
	if types.ByteSize(fileinfo.Size) > maxSize {
		return fileinfo.Name, "", fileinfo.MimeType,
			errors.Errorf("Maximum attachment size %v exceeded, file size %v", maxSize, types.ByteSize(fileinfo.Size))
	}

	fileBytes, err := mmClient.File.GetByPath(fileinfo.Path)
	if err != nil {
		return "", "", "", err
	}
	attachment, err := client.RESTPostAttachment(issueKey, fileBytes, fileinfo.Name)
	if err != nil {
		return fileinfo.Name, "", fileinfo.MimeType, err
	}

	return fileinfo.Name, attachment.Filename, fileinfo.MimeType, nil
}

// GetSelf returns a user associated with this Jira client
func (client JiraClient) GetSelf() (*jira.User, error) {
	self, resp, err := client.Jira.User.GetSelf()
	if err != nil {
		return nil, userFriendlyJiraError(resp, err)
	}
	return self, nil
}

// MakeCreateIssueURL makes a URL that would take a browser to a pre-filled form
// to file a new issue in Jira.
func MakeCreateIssueURL(instance Instance, project *jira.Project, issue *jira.Issue) string {
	url, err := url.Parse(fmt.Sprintf("%v/secure/CreateIssueDetails!init.jspa", instance.GetJiraBaseURL()))
	if err != nil {
		return ""
	}

	q := url.Query()
	q.Add("pid", project.ID)
	q.Add("issuetype", issue.Fields.Type.ID)
	q.Add("summary", issue.Fields.Summary)
	q.Add("description", issue.Fields.Description)

	// Add reporter for only server instances
	if instance.Common().Type == ServerInstanceType && issue.Fields.Reporter != nil {
		q.Add("reporter", issue.Fields.Reporter.Name)
	}

	// if no priority, ID field does not exist
	if issue.Fields.Priority != nil {
		q.Add("priority", issue.Fields.Priority.ID)
	}

	// add custom fields
	for key, field := range issue.Fields.Unknowns {
		switch fieldTyped := field.(type) {
		case string: // handles fields like url, short text, paragraph
			q.Add(key, fieldTyped)
		case map[string]interface{}: // handles fields like dropdown
			if id, ok := fieldTyped["id"].(string); ok {
				q.Add(key, id)
			}
		case []interface{}: // handles fields like labels, checkbox, flags
			for _, element := range fieldTyped {
				switch elementTyped := element.(type) {
				case string:
					q.Add(key, elementTyped)
				case map[string]interface{}:
					if id, ok := elementTyped["id"].(string); ok {
						q.Add(key, id)
					}
				case []map[string]interface{}:
					for _, mapWithID := range elementTyped {
						if id, ok := mapWithID["id"].(string); ok {
							q.Add(key, id)
						}
					}
				}
			}
		}
	}

	url.RawQuery = q.Encode()
	return url.String()
}

// SearchUsersAssignableToIssue finds all users that can be assigned to an issue.
// This is the shared implementation between the Server and the Cloud versions
// which use different queryKey's.
func SearchUsersAssignableToIssue(client Client, issueKey, queryKey, queryValue string, maxResults int) ([]jira.User, error) {
	users := []jira.User{}
	params := map[string]string{
		"issueKey": issueKey,
		queryKey:   queryValue,
	}
	if maxResults > 0 {
		params["maxResults"] = strconv.Itoa(maxResults)
	}
	err := client.RESTGet(userSearchRoute, params, &users)
	if err != nil {
		return nil, err
	}
	return users, nil
}

// SearchUsersAssignableInProject finds all users that can be assigned to some issue in a given project.
// This is the shared implementation between the Server and the Cloud versions
// which use different queryKey's.
func SearchUsersAssignableInProject(client Client, projectKey, queryKey, queryValue string, maxResults int) ([]jira.User, error) {
	users := []jira.User{}
	params := map[string]string{
		"project": projectKey,
		queryKey:  queryValue,
	}
	if maxResults > 0 {
		params["maxResults"] = strconv.Itoa(maxResults)
	}
	err := client.RESTGet(userSearchRoute, params, &users)
	if err != nil {
		return nil, err
	}
	return users, nil
}

func endpointURL(endpoint string) (string, error) {
	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	if parsedURL.Scheme == "" {
		// relative path
		endpoint = path.Join("/rest/api", endpoint)
	}
	return endpoint, nil
}

var keyOrIDRegex = regexp.MustCompile("(^[[:alnum:]]+-)?[[:digit:]]+$")

func endpointNameFromRequest(r *http.Request) string {
	_, path := splitInstancePath(r.URL.Path)
	l := strings.ToLower(path)
	s := strings.TrimPrefix(l, "/rest/api")
	s = strings.Trim(s, "/")

	if s == l {
		return "_unrecognized"
	}

	parts := strings.Split(s, "/")
	n := len(parts)

	if n < 2 {
		return unrecognizedEndpoint
	}
	var out = []string{"api/jira", parts[0], parts[1]}
	context := parts[1]
	for _, p := range parts[2:] {
		switch context {
		case "issue":
			if keyOrIDRegex.MatchString(p) {
				continue
			}

		case "user":
			if p != "groups" && p != "assignable" {
				continue
			}

		case "project", "comment":
			continue
		}
		out = append(out, p)
		context = p
	}

	out = append(out, r.Method)
	return strings.Join(out, "/")
}

// RESTError is an error type that embeds the http response status code, and implements a
// StatusCoder interface to access it
type RESTError struct {
	error
	Status int
}

// StatusCoder is an interface to access the HTTP response status code value
type StatusCoder interface {
	StatusCode() int
}

// StatusCode returns the HTTP status code embedded in the error
func (e RESTError) StatusCode() int {
	return e.Status
}

// StatusCode is a convenience function that returns the status code if err implements a
// StatusCoder, otherwise it returns http.StatusOK/http.StatusInternalServerError depending
// on the err value.
func StatusCode(err error) int {
	if err == nil {
		return http.StatusOK
	}
	coder, ok := err.(StatusCoder)
	if !ok {
		return http.StatusInternalServerError
	}
	return coder.StatusCode()
}

func userFriendlyJiraError(resp *jira.Response, err error) error {
	jerr, ok := err.(*jira.Error)
	if !ok {
		if resp == nil {
			return RESTError{err, http.StatusInternalServerError}
		}
		err = jira.NewJiraError(resp, err)
		jerr, ok = err.(*jira.Error)
		if !ok {
			return RESTError{err, resp.StatusCode}
		}
	}
	if len(jerr.Errors) == 0 && len(jerr.ErrorMessages) == 0 {
		return RESTError{err, resp.StatusCode}
	}

	message := ""
	for k, v := range jerr.Errors {
		message += fmt.Sprintf(" - %s: %s\n", k, v)
	}
	for _, m := range jerr.ErrorMessages {
		message += fmt.Sprintf(" - %s\n", m)
	}

	if resp != nil {
		return RESTError{errors.New(message), resp.StatusCode}
	}
	return RESTError{errors.New(message), 0}
}
