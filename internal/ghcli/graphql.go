package ghcli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mitsuhiko/gh-issue-sync/internal/issue"
)

// IssueRelationships holds the parent, blocking, issue type, and project data for an issue.
type IssueRelationships struct {
	Parent    *issue.IssueRef
	BlockedBy []issue.IssueRef
	Blocks    []issue.IssueRef
	IssueType string
	Projects  []string
}

// graphqlIssue represents the GraphQL response structure for an issue.
type graphqlIssue struct {
	ID        string `json:"id"`
	Number    int    `json:"number"`
	IssueType *struct {
		Name string `json:"name"`
	} `json:"issueType"`
	ProjectItems *struct {
		Nodes []struct {
			Project struct {
				Title string `json:"title"`
			} `json:"project"`
		} `json:"nodes"`
	} `json:"projectItems"`
	Parent *struct {
		Number int    `json:"number"`
		ID     string `json:"id"`
	} `json:"parent"`
	BlockedBy struct {
		Nodes []struct {
			Number int    `json:"number"`
			ID     string `json:"id"`
		} `json:"nodes"`
	} `json:"blockedBy"`
	Blocking struct {
		Nodes []struct {
			Number int    `json:"number"`
			ID     string `json:"id"`
		} `json:"nodes"`
	} `json:"blocking"`
}

type graphqlResponse struct {
	Data struct {
		Repository struct {
			Issue graphqlIssue `json:"issue"`
		} `json:"repository"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

type graphqlMutationResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// GetIssueRelationships fetches parent and blocking relationships for an issue via GraphQL.
func (c *Client) GetIssueRelationships(ctx context.Context, number string) (IssueRelationships, string, error) {
	results, err := c.GetIssueRelationshipsBatch(ctx, []string{number})
	if err != nil {
		return IssueRelationships{}, "", err
	}
	if rel, ok := results[number]; ok {
		return rel, "", nil // Note: we don't return the ID anymore, but it's not used
	}
	return IssueRelationships{}, "", fmt.Errorf("issue %s not found in response", number)
}

// GetIssueRelationshipsBatch fetches parent and blocking relationships for multiple issues
// in a single GraphQL call. Returns a map of issue number -> relationships.
func (c *Client) GetIssueRelationshipsBatch(ctx context.Context, numbers []string) (map[string]IssueRelationships, error) {
	if len(numbers) == 0 {
		return map[string]IssueRelationships{}, nil
	}

	owner, repo := splitRepo(c.repo)
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("invalid repository format")
	}

	// Build a batched GraphQL query with aliases for each issue
	// GraphQL aliases allow us to fetch multiple issues in one query:
	// query { repository(owner: "x", name: "y") { issue1: issue(number: 1) { ... } issue2: issue(number: 2) { ... } } }
	var issueQueries []string
	for i, num := range numbers {
		n, err := strconv.Atoi(num)
		if err != nil {
			continue // Skip invalid numbers
		}
		issueQueries = append(issueQueries, fmt.Sprintf(`issue%d: issue(number: %d) {
      id
      number
      issueType { name }
      projectItems(first: 20) {
        nodes {
          project { title }
        }
      }
      parent {
        number
        id
      }
      blockedBy(first: 100) {
        nodes {
          number
          id
        }
      }
      blocking(first: 100) {
        nodes {
          number
          id
        }
      }
    }`, i, n))
	}

	if len(issueQueries) == 0 {
		return map[string]IssueRelationships{}, nil
	}

	query := fmt.Sprintf(`query($owner: String!, $repo: String!) {
  repository(owner: $owner, name: $repo) {
    %s
  }
}`, strings.Join(issueQueries, "\n    "))

	args := []string{"api", "graphql",
		"-f", fmt.Sprintf("query=%s", query),
		"-F", fmt.Sprintf("owner=%s", owner),
		"-F", fmt.Sprintf("repo=%s", repo),
	}

	out, err := c.runner.Run(ctx, "gh", args...)
	if err != nil {
		return nil, err
	}

	// Parse the response - we need a dynamic structure since aliases are dynamic
	var resp struct {
		Data struct {
			Repository map[string]json.RawMessage `json:"repository"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse GraphQL response: %w", err)
	}

	if len(resp.Errors) > 0 {
		return nil, fmt.Errorf("GraphQL error: %s", resp.Errors[0].Message)
	}

	results := make(map[string]IssueRelationships)

	// Parse each aliased issue response
	for alias, rawIssue := range resp.Data.Repository {
		if !strings.HasPrefix(alias, "issue") {
			continue
		}
		if string(rawIssue) == "null" {
			continue
		}

		var issueData graphqlIssue
		if err := json.Unmarshal(rawIssue, &issueData); err != nil {
			continue // Skip malformed issues
		}

		rels := IssueRelationships{}
		if issueData.IssueType != nil {
			rels.IssueType = issueData.IssueType.Name
		}
		if issueData.ProjectItems != nil {
			for _, node := range issueData.ProjectItems.Nodes {
				rels.Projects = append(rels.Projects, node.Project.Title)
			}
		}
		if issueData.Parent != nil {
			ref := issue.IssueRef(strconv.Itoa(issueData.Parent.Number))
			rels.Parent = &ref
		}
		for _, node := range issueData.BlockedBy.Nodes {
			rels.BlockedBy = append(rels.BlockedBy, issue.IssueRef(strconv.Itoa(node.Number)))
		}
		for _, node := range issueData.Blocking.Nodes {
			rels.Blocks = append(rels.Blocks, issue.IssueRef(strconv.Itoa(node.Number)))
		}

		results[strconv.Itoa(issueData.Number)] = rels
	}

	return results, nil
}

// GetIssueNodeID fetches the GraphQL node ID for an issue.
func (c *Client) GetIssueNodeID(ctx context.Context, number string) (string, error) {
	owner, repo := splitRepo(c.repo)
	if owner == "" || repo == "" {
		return "", fmt.Errorf("invalid repository format")
	}

	query := `
query($owner: String!, $repo: String!, $number: Int!) {
  repository(owner: $owner, name: $repo) {
    issue(number: $number) {
      id
    }
  }
}`

	num, err := strconv.Atoi(number)
	if err != nil {
		return "", fmt.Errorf("invalid issue number: %s", number)
	}

	args := []string{"api", "graphql",
		"-f", fmt.Sprintf("query=%s", query),
		"-F", fmt.Sprintf("owner=%s", owner),
		"-F", fmt.Sprintf("repo=%s", repo),
		"-F", fmt.Sprintf("number=%d", num),
	}

	out, err := c.runner.Run(ctx, "gh", args...)
	if err != nil {
		return "", err
	}

	var resp graphqlResponse
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return "", fmt.Errorf("failed to parse GraphQL response: %w", err)
	}

	if len(resp.Errors) > 0 {
		return "", fmt.Errorf("GraphQL error: %s", resp.Errors[0].Message)
	}

	return resp.Data.Repository.Issue.ID, nil
}

// SetParent sets or removes the parent of an issue.
// If parentNumber is empty, the parent relationship is removed.
func (c *Client) SetParent(ctx context.Context, issueNumber string, parentNumber string) error {
	if parentNumber == "" {
		return c.removeParent(ctx, issueNumber)
	}

	parentNodeID, err := c.GetIssueNodeID(ctx, parentNumber)
	if err != nil {
		return fmt.Errorf("failed to get parent issue node ID: %w", err)
	}

	childNodeID, err := c.GetIssueNodeID(ctx, issueNumber)
	if err != nil {
		return fmt.Errorf("failed to get child issue node ID: %w", err)
	}

	mutation := `
mutation($parentId: ID!, $childId: ID!) {
  addSubIssue(input: {issueId: $parentId, subIssueId: $childId, replaceParent: true}) {
    issue {
      number
    }
  }
}`

	args := []string{"api", "graphql",
		"-f", fmt.Sprintf("query=%s", mutation),
		"-f", fmt.Sprintf("parentId=%s", parentNodeID),
		"-f", fmt.Sprintf("childId=%s", childNodeID),
	}

	out, err := c.runner.Run(ctx, "gh", args...)
	if err != nil {
		return err
	}

	var resp graphqlMutationResponse
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return fmt.Errorf("failed to parse GraphQL response: %w", err)
	}

	if len(resp.Errors) > 0 {
		return fmt.Errorf("GraphQL error: %s", resp.Errors[0].Message)
	}

	return nil
}

// removeParent removes the parent relationship from an issue.
func (c *Client) removeParent(ctx context.Context, issueNumber string) error {
	// First, get the current parent
	rels, childNodeID, err := c.GetIssueRelationships(ctx, issueNumber)
	if err != nil {
		return fmt.Errorf("failed to get issue relationships: %w", err)
	}

	if rels.Parent == nil {
		// No parent to remove
		return nil
	}

	parentNodeID, err := c.GetIssueNodeID(ctx, rels.Parent.String())
	if err != nil {
		return fmt.Errorf("failed to get parent issue node ID: %w", err)
	}

	mutation := `
mutation($parentId: ID!, $childId: ID!) {
  removeSubIssue(input: {issueId: $parentId, subIssueId: $childId}) {
    issue {
      number
    }
  }
}`

	args := []string{"api", "graphql",
		"-f", fmt.Sprintf("query=%s", mutation),
		"-f", fmt.Sprintf("parentId=%s", parentNodeID),
		"-f", fmt.Sprintf("childId=%s", childNodeID),
	}

	out, err := c.runner.Run(ctx, "gh", args...)
	if err != nil {
		return err
	}

	var resp graphqlMutationResponse
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return fmt.Errorf("failed to parse GraphQL response: %w", err)
	}

	if len(resp.Errors) > 0 {
		return fmt.Errorf("GraphQL error: %s", resp.Errors[0].Message)
	}

	return nil
}

// AddBlockedBy adds a blocking relationship (issueNumber is blocked by blockingNumber).
func (c *Client) AddBlockedBy(ctx context.Context, issueNumber string, blockingNumber string) error {
	issueNodeID, err := c.GetIssueNodeID(ctx, issueNumber)
	if err != nil {
		return fmt.Errorf("failed to get issue node ID: %w", err)
	}

	blockingNodeID, err := c.GetIssueNodeID(ctx, blockingNumber)
	if err != nil {
		return fmt.Errorf("failed to get blocking issue node ID: %w", err)
	}

	mutation := `
mutation($issueId: ID!, $blockingId: ID!) {
  addBlockedBy(input: {issueId: $issueId, blockingIssueId: $blockingId}) {
    issue {
      number
    }
    blockingIssue {
      number
    }
  }
}`

	args := []string{"api", "graphql",
		"-f", fmt.Sprintf("query=%s", mutation),
		"-f", fmt.Sprintf("issueId=%s", issueNodeID),
		"-f", fmt.Sprintf("blockingId=%s", blockingNodeID),
	}

	out, err := c.runner.Run(ctx, "gh", args...)
	if err != nil {
		return err
	}

	var resp graphqlMutationResponse
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return fmt.Errorf("failed to parse GraphQL response: %w", err)
	}

	if len(resp.Errors) > 0 {
		return fmt.Errorf("GraphQL error: %s", resp.Errors[0].Message)
	}

	return nil
}

// RemoveBlockedBy removes a blocking relationship (issueNumber is no longer blocked by blockingNumber).
func (c *Client) RemoveBlockedBy(ctx context.Context, issueNumber string, blockingNumber string) error {
	issueNodeID, err := c.GetIssueNodeID(ctx, issueNumber)
	if err != nil {
		return fmt.Errorf("failed to get issue node ID: %w", err)
	}

	blockingNodeID, err := c.GetIssueNodeID(ctx, blockingNumber)
	if err != nil {
		return fmt.Errorf("failed to get blocking issue node ID: %w", err)
	}

	mutation := `
mutation($issueId: ID!, $blockingId: ID!) {
  removeBlockedBy(input: {issueId: $issueId, blockingIssueId: $blockingId}) {
    issue {
      number
    }
    blockingIssue {
      number
    }
  }
}`

	args := []string{"api", "graphql",
		"-f", fmt.Sprintf("query=%s", mutation),
		"-f", fmt.Sprintf("issueId=%s", issueNodeID),
		"-f", fmt.Sprintf("blockingId=%s", blockingNodeID),
	}

	out, err := c.runner.Run(ctx, "gh", args...)
	if err != nil {
		return err
	}

	var resp graphqlMutationResponse
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return fmt.Errorf("failed to parse GraphQL response: %w", err)
	}

	if len(resp.Errors) > 0 {
		return fmt.Errorf("GraphQL error: %s", resp.Errors[0].Message)
	}

	return nil
}

// SyncRelationships syncs the parent and blocking relationships for an issue.
// It compares the desired state (from local issue) with the current remote state
// and makes the necessary mutations.
func (c *Client) SyncRelationships(ctx context.Context, issueNumber string, local issue.Issue) error {
	// Get current remote relationships
	remote, _, err := c.GetIssueRelationships(ctx, issueNumber)
	if err != nil {
		return fmt.Errorf("failed to get remote relationships: %w", err)
	}

	// Sync parent
	localParent := ""
	if local.Parent != nil {
		localParent = local.Parent.String()
	}
	remoteParent := ""
	if remote.Parent != nil {
		remoteParent = remote.Parent.String()
	}

	if localParent != remoteParent {
		if err := c.SetParent(ctx, issueNumber, localParent); err != nil {
			return fmt.Errorf("failed to set parent: %w", err)
		}
	}

	// Sync blocked_by
	localBlockedBy := make(map[string]struct{})
	for _, ref := range local.BlockedBy {
		if !ref.IsLocal() {
			localBlockedBy[ref.String()] = struct{}{}
		}
	}
	remoteBlockedBy := make(map[string]struct{})
	for _, ref := range remote.BlockedBy {
		remoteBlockedBy[ref.String()] = struct{}{}
	}

	// Add new blocked_by relationships
	for ref := range localBlockedBy {
		if _, ok := remoteBlockedBy[ref]; !ok {
			if err := c.AddBlockedBy(ctx, issueNumber, ref); err != nil {
				return fmt.Errorf("failed to add blocked_by %s: %w", ref, err)
			}
		}
	}

	// Remove old blocked_by relationships
	for ref := range remoteBlockedBy {
		if _, ok := localBlockedBy[ref]; !ok {
			if err := c.RemoveBlockedBy(ctx, issueNumber, ref); err != nil {
				return fmt.Errorf("failed to remove blocked_by %s: %w", ref, err)
			}
		}
	}

	// Note: We don't directly sync "blocks" because it's the inverse of "blocked_by".
	// If issue A blocks issue B, that means B is blocked_by A.
	// The "blocks" field in our local issue is informational and derived from the
	// blocked_by relationships of other issues.
	//
	// However, if the user explicitly sets "blocks" on an issue, we should add
	// the corresponding blocked_by relationship on the target issues.
	localBlocks := make(map[string]struct{})
	for _, ref := range local.Blocks {
		if !ref.IsLocal() {
			localBlocks[ref.String()] = struct{}{}
		}
	}
	remoteBlocks := make(map[string]struct{})
	for _, ref := range remote.Blocks {
		remoteBlocks[ref.String()] = struct{}{}
	}

	// Add new blocks relationships (by adding blocked_by on the target)
	for ref := range localBlocks {
		if _, ok := remoteBlocks[ref]; !ok {
			if err := c.AddBlockedBy(ctx, ref, issueNumber); err != nil {
				return fmt.Errorf("failed to add blocks %s: %w", ref, err)
			}
		}
	}

	// Remove old blocks relationships (by removing blocked_by on the target)
	for ref := range remoteBlocks {
		if _, ok := localBlocks[ref]; !ok {
			if err := c.RemoveBlockedBy(ctx, ref, issueNumber); err != nil {
				return fmt.Errorf("failed to remove blocks %s: %w", ref, err)
			}
		}
	}

	return nil
}

// splitRepo splits "owner/repo" into owner and repo parts.
func splitRepo(repo string) (string, string) {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}
