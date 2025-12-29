package issue

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type IssueNumber string

type IssueRef string

type Issue struct {
	Number      IssueNumber
	Title       string
	Labels      []string
	Assignees   []string
	Milestone   string
	State       string
	StateReason *string
	Parent      *IssueRef
	BlockedBy   []IssueRef
	Blocks      []IssueRef
	SyncedAt    *time.Time
	Body        string
}

type FrontMatter struct {
	Number      IssueNumber `yaml:"number"`
	Title       string      `yaml:"title"`
	Labels      []string    `yaml:"labels,omitempty"`
	Assignees   []string    `yaml:"assignees,omitempty"`
	Milestone   string      `yaml:"milestone,omitempty"`
	State       string      `yaml:"state,omitempty"`
	StateReason *string     `yaml:"state_reason"`
	Parent      *IssueRef   `yaml:"parent,omitempty"`
	BlockedBy   []IssueRef  `yaml:"blocked_by,omitempty"`
	Blocks      []IssueRef  `yaml:"blocks,omitempty"`
	SyncedAt    *time.Time  `yaml:"synced_at,omitempty"`
}

func (n IssueNumber) String() string {
	return string(n)
}

func (n IssueNumber) IsLocal() bool {
	return strings.HasPrefix(string(n), "T")
}

func (n *IssueNumber) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.ScalarNode {
		return fmt.Errorf("issue number must be scalar")
	}
	if value.Tag == "!!int" {
		*n = IssueNumber(value.Value)
		return nil
	}
	*n = IssueNumber(value.Value)
	return nil
}

func (n IssueNumber) MarshalYAML() (interface{}, error) {
	if n == "" {
		return nil, nil
	}
	if n.IsLocal() {
		return string(n), nil
	}
	parsed, err := strconv.Atoi(string(n))
	if err != nil {
		return string(n), nil
	}
	return parsed, nil
}

func (r IssueRef) String() string {
	return string(r)
}

func (r IssueRef) IsLocal() bool {
	return strings.HasPrefix(string(r), "T")
}

func (r *IssueRef) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.ScalarNode {
		return fmt.Errorf("issue reference must be scalar")
	}
	*r = IssueRef(value.Value)
	return nil
}

func (r IssueRef) MarshalYAML() (interface{}, error) {
	if r == "" {
		return nil, nil
	}
	if r.IsLocal() {
		return string(r), nil
	}
	parsed, err := strconv.Atoi(string(r))
	if err != nil {
		return string(r), nil
	}
	return parsed, nil
}

var frontMatterDelimiter = []byte("---")

func ParseFile(path string) (Issue, error) {
	data, err := osReadFile(path)
	if err != nil {
		return Issue{}, err
	}
	return Parse(data)
}

func Parse(data []byte) (Issue, error) {
	frontMatter, body, err := splitFrontMatter(data)
	if err != nil {
		return Issue{}, err
	}
	var fm FrontMatter
	if err := yaml.Unmarshal(frontMatter, &fm); err != nil {
		return Issue{}, err
	}
	issue := Issue{
		Number:      fm.Number,
		Title:       fm.Title,
		Labels:      fm.Labels,
		Assignees:   fm.Assignees,
		Milestone:   fm.Milestone,
		State:       fm.State,
		StateReason: fm.StateReason,
		Parent:      fm.Parent,
		BlockedBy:   fm.BlockedBy,
		Blocks:      fm.Blocks,
		SyncedAt:    fm.SyncedAt,
		Body:        normalizeBody(string(body)),
	}
	return issue, nil
}

func Render(issue Issue) (string, error) {
	fm := FrontMatter{
		Number:      issue.Number,
		Title:       issue.Title,
		Labels:      sortedStrings(issue.Labels),
		Assignees:   sortedStrings(issue.Assignees),
		Milestone:   issue.Milestone,
		State:       issue.State,
		StateReason: issue.StateReason,
		Parent:      issue.Parent,
		BlockedBy:   sortedRefs(issue.BlockedBy),
		Blocks:      sortedRefs(issue.Blocks),
		SyncedAt:    issue.SyncedAt,
	}
	payload, err := yaml.Marshal(&fm)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	buf.Write(frontMatterDelimiter)
	buf.WriteByte('\n')
	buf.Write(payload)
	buf.Write(frontMatterDelimiter)
	buf.WriteByte('\n')
	buf.WriteByte('\n')
	buf.WriteString(normalizeBody(issue.Body))
	return buf.String(), nil
}

func WriteFile(path string, issue Issue) error {
	content, err := Render(issue)
	if err != nil {
		return err
	}
	return osWriteFile(path, []byte(content), 0o644)
}

func FileName(number IssueNumber, title string) string {
	slug := Slugify(title)
	if slug == "" {
		slug = "issue"
	}
	return fmt.Sprintf("%s-%s.md", number, slug)
}

func PathFor(dir string, number IssueNumber, title string) string {
	return filepath.Join(dir, FileName(number, title))
}

func Normalize(issue Issue) Issue {
	issue.Labels = sortedStrings(issue.Labels)
	issue.Assignees = sortedStrings(issue.Assignees)
	issue.BlockedBy = sortedRefs(issue.BlockedBy)
	issue.Blocks = sortedRefs(issue.Blocks)
	issue.Body = normalizeBody(issue.Body)
	return issue
}

func EqualIgnoringSyncedAt(a, b Issue) bool {
	a = Normalize(a)
	b = Normalize(b)
	a.SyncedAt = nil
	b.SyncedAt = nil

	if a.Number != b.Number {
		return false
	}
	if a.Title != b.Title {
		return false
	}
	if !stringSlicesEqual(a.Labels, b.Labels) {
		return false
	}
	if !stringSlicesEqual(a.Assignees, b.Assignees) {
		return false
	}
	if a.Milestone != b.Milestone {
		return false
	}
	if a.State != b.State {
		return false
	}
	if normalizeOptional(a.StateReason) != normalizeOptional(b.StateReason) {
		return false
	}
	if normalizeOptionalRef(a.Parent) != normalizeOptionalRef(b.Parent) {
		return false
	}
	if !refSlicesEqual(a.BlockedBy, b.BlockedBy) {
		return false
	}
	if !refSlicesEqual(a.Blocks, b.Blocks) {
		return false
	}
	if a.Body != b.Body {
		return false
	}
	return true
}

func normalizeOptional(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func normalizeOptionalRef(value *IssueRef) string {
	if value == nil {
		return ""
	}
	return value.String()
}

func normalizeBody(body string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.TrimLeft(body, "\n")
	if body == "" {
		return ""
	}
	if !strings.HasSuffix(body, "\n") {
		return body + "\n"
	}
	return body
}

func sortedStrings(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	cleaned := make([]string, 0, len(items))
	seen := make(map[string]struct{})
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		cleaned = append(cleaned, item)
	}
	sort.Strings(cleaned)
	return cleaned
}

func sortedRefs(items []IssueRef) []IssueRef {
	if len(items) == 0 {
		return nil
	}
	cleaned := make([]IssueRef, 0, len(items))
	seen := make(map[string]struct{})
	for _, item := range items {
		key := strings.TrimSpace(item.String())
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		cleaned = append(cleaned, IssueRef(key))
	}
	sort.Slice(cleaned, func(i, j int) bool {
		return cleaned[i].String() < cleaned[j].String()
	})
	return cleaned
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func refSlicesEqual(a, b []IssueRef) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func splitFrontMatter(data []byte) ([]byte, []byte, error) {
	if bytes.HasPrefix(data, []byte("\xef\xbb\xbf")) {
		data = data[3:]
	}
	if !bytes.HasPrefix(data, append(frontMatterDelimiter, '\n')) {
		return nil, nil, errors.New("missing front matter")
	}
	lines := bytes.Split(data, []byte("\n"))
	end := -1
	for i := 1; i < len(lines); i++ {
		if bytes.Equal(lines[i], frontMatterDelimiter) {
			end = i
			break
		}
	}
	if end == -1 {
		return nil, nil, errors.New("unterminated front matter")
	}
	front := bytes.Join(lines[1:end], []byte("\n"))
	body := bytes.Join(lines[end+1:], []byte("\n"))
	body = bytes.TrimPrefix(body, []byte("\n"))
	return front, body, nil
}

var slugRegex = regexp.MustCompile(`[^a-z0-9]+`)

func Slugify(title string) string {
	lower := strings.ToLower(strings.TrimSpace(title))
	if lower == "" {
		return ""
	}
	slug := slugRegex.ReplaceAllString(lower, "-")
	slug = strings.Trim(slug, "-")
	slug = strings.Trim(slug, ".")
	slug = strings.ReplaceAll(slug, "--", "-")
	return slug
}

// osReadFile and osWriteFile are swapped out in tests.
var osReadFile = func(path string) ([]byte, error) {
	return os.ReadFile(path)
}

var osWriteFile = func(path string, data []byte, perm os.FileMode) error {
	return os.WriteFile(path, data, perm)
}
