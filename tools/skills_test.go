package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// newSkillsClientSession connects an in-memory client that can send
// skills/list and skills/get to a server built by NewServer.
func newSkillsClientSession(t *testing.T) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	ss, err := NewServer("0.0.0").Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server.Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = ss.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.0"}, nil)
	if err := mcp.AddSendingCustomMethod[*ListSkillsParams, *ListSkillsResult](client, "skills/list"); err != nil {
		t.Fatal(err)
	}
	if err := mcp.AddSendingCustomMethod[*GetSkillParams, *GetSkillResult](client, "skills/get"); err != nil {
		t.Fatal(err)
	}
	cs, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// TestSkillsOverMCP checks the server side of the Skills extension as a host
// sees it: the capability, skills/list, the manifest against the bytes
// resources/read serves, and skills/get.
func TestSkillsOverMCP(t *testing.T) {
	cs := newSkillsClientSession(t)
	ctx := context.Background()

	caps := cs.InitializeResult().Capabilities
	if caps == nil || caps.Extensions[SkillsExtension] == nil {
		t.Fatalf("capabilities do not advertise the %s extension", SkillsExtension)
	}
	if caps.Resources == nil {
		t.Error("capabilities do not advertise resources, which the Skills extension requires")
	}

	list, err := mcp.CallCustomMethod[*ListSkillsParams, *ListSkillsResult](ctx, cs, "skills/list", &ListSkillsParams{})
	if err != nil {
		t.Fatalf("skills/list error = %v", err)
	}
	if len(list.Skills) != 1 || list.Skills[0].URI != "skill://smartmeter-dashboard/SKILL.md" {
		t.Fatalf("skills/list = %+v, want only skill://smartmeter-dashboard/SKILL.md", list.Skills)
	}

	for _, skill := range list.Skills {
		if skill.Frontmatter["name"] == "" || skill.Frontmatter["description"] == "" {
			t.Errorf("%s: frontmatter = %v, want name and description", skill.URI, skill.Frontmatter)
		}
		if len(skill.Resources) == 0 || skill.Resources[0].URI != skill.URI {
			t.Errorf("%s: manifest must list SKILL.md first, got %+v", skill.URI, skill.Resources)
		}
		for _, r := range skill.Resources {
			rr, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: r.URI})
			if err != nil {
				t.Errorf("ReadResource(%s) error = %v", r.URI, err)
				continue
			}
			if len(rr.Contents) != 1 {
				t.Errorf("ReadResource(%s) returned %d contents, want 1", r.URI, len(rr.Contents))
				continue
			}
			body := []byte(rr.Contents[0].Text)
			sum := sha256.Sum256(body)
			if got := "sha256:" + hex.EncodeToString(sum[:]); got != r.Digest {
				t.Errorf("%s: served digest %s, manifest %s", r.URI, got, r.Digest)
			}
			if len(body) != r.Size {
				t.Errorf("%s: served %d bytes, manifest %d", r.URI, len(body), r.Size)
			}
			if rr.Contents[0].MIMEType != "text/markdown" {
				t.Errorf("%s: MIMEType = %q, want text/markdown", r.URI, rr.Contents[0].MIMEType)
			}
		}

		got, err := mcp.CallCustomMethod[*GetSkillParams, *GetSkillResult](ctx, cs, "skills/get", &GetSkillParams{URI: skill.URI})
		if err != nil {
			t.Errorf("skills/get(%s) error = %v", skill.URI, err)
			continue
		}
		if got.Skill == nil || got.Skill.URI != skill.URI || len(got.Skill.Resources) != len(skill.Resources) {
			t.Errorf("skills/get(%s) = %+v, want the skills/list entry", skill.URI, got.Skill)
		}
	}

	for _, uri := range []string{"skill://unknown/SKILL.md", "skill://smartmeter-dashboard/references/troubleshooting.md", ""} {
		_, err := mcp.CallCustomMethod[*GetSkillParams, *GetSkillResult](ctx, cs, "skills/get", &GetSkillParams{URI: uri})
		var rpcErr *jsonrpc.Error
		if !errors.As(err, &rpcErr) || rpcErr.Code != jsonrpc.CodeInvalidParams {
			t.Errorf("skills/get(%q) error = %v, want code %d", uri, err, jsonrpc.CodeInvalidParams)
		}
	}
}

// TestSkillReferences checks that what the published skills point at exists:
// relative file references resolve to a file of the same skill, and every
// backquoted tool name is a registered tool.
func TestSkillReferences(t *testing.T) {
	cs := newRegisteredClientSession(t)
	tl, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	toolNames := map[string]bool{}
	for _, tool := range tl.Tools {
		toolNames[tool.Name] = true
	}

	fileRef := regexp.MustCompile("`(references/[^`]+)`")
	toolRef := regexp.MustCompile("`([a-z]+(?:_[a-z]+)+)`")
	for _, skill := range publishedSkills.skills {
		inManifest := map[string]bool{}
		for _, r := range skill.Resources {
			inManifest[r.URI] = true
		}
		root := strings.TrimSuffix(skill.URI, "SKILL.md")
		for _, f := range publishedSkills.files {
			if !strings.HasPrefix(f.uri, root) {
				continue
			}
			for _, m := range fileRef.FindAllStringSubmatch(string(f.data), -1) {
				if !inManifest[root+m[1]] {
					t.Errorf("%s refers to %s, which is not a file of the skill", f.uri, m[1])
				}
			}
			for _, m := range toolRef.FindAllStringSubmatch(string(f.data), -1) {
				if !toolNames[m[1]] && !strings.HasSuffix(m[1], "_sec") {
					t.Errorf("%s refers to tool %s, which is not registered", f.uri, m[1])
				}
			}
		}
	}
}

func TestLoadSkillsRejectsInvalidSkills(t *testing.T) {
	tests := []struct {
		name string
		fsys fstest.MapFS
	}{
		{"name mismatch", fstest.MapFS{"skills/a/SKILL.md": {Data: []byte("---\nname: b\ndescription: d\n---\n")}}},
		{"missing description", fstest.MapFS{"skills/a/SKILL.md": {Data: []byte("---\nname: a\n---\n")}}},
		{"missing SKILL.md", fstest.MapFS{"skills/a/README.md": {Data: []byte("x")}}},
		{"file at top level", fstest.MapFS{"skills/README.md": {Data: []byte("x")}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := loadSkills(tt.fsys, "skills"); err == nil {
				t.Error("loadSkills() error = nil, want an error")
			}
		})
	}
}

func TestParseFrontmatter(t *testing.T) {
	tests := []struct {
		name    string
		md      string
		want    map[string]string
		wantErr bool
	}{
		{"flat fields", "---\nname: a\ndescription: does (render_*_ui) things\n---\nbody", map[string]string{"name": "a", "description": "does (render_*_ui) things"}, false},
		{"no frontmatter", "# title\n", nil, true},
		{"unterminated", "---\nname: a\n", nil, true},
		{"nested value", "---\nname: a\nmetadata:\n  k: v\n---\n", nil, true},
		{"colon in value", "---\nname: a\ndescription: x: y\n---\n", nil, true},
		{"quoted value", "---\nname: \"a\"\n---\n", nil, true},
		{"duplicate key", "---\nname: a\nname: b\n---\n", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseFrontmatter([]byte(tt.md))
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseFrontmatter() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("parseFrontmatter() = %v, want %v", got, tt.want)
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("parseFrontmatter()[%q] = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}
