package tools

import (
	"bufio"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// SkillsExtension is the identifier of the Skills over MCP extension
// (SEP-2640).
const SkillsExtension = "io.modelcontextprotocol/skills"

// skillURIScheme is the URI scheme the Skills extension recommends for skill
// files: skill://<skill-name>/<path>.
const skillURIScheme = "skill://"

// skillsFS holds the skills el-mcp-server publishes. Each top-level directory
// is one skill: a SKILL.md with YAML frontmatter plus optional supporting
// files.
//
//go:embed all:skills
var skillsFS embed.FS

// SkillResource is one file of a skill's manifest.
type SkillResource struct {
	URI    string `json:"uri"`
	Digest string `json:"digest"`
	Size   int    `json:"size"`
}

// Skill is a skills/list entry: the SKILL.md URI, its frontmatter and the
// manifest of every file of the skill, SKILL.md included.
type Skill struct {
	URI         string            `json:"uri"`
	Frontmatter map[string]string `json:"frontmatter"`
	Resources   []SkillResource   `json:"resources"`
}

// ListSkillsParams are the params of skills/list.
type ListSkillsParams struct {
	mcp.ParamsBase
	Cursor string `json:"cursor,omitempty"`
}

// ListSkillsResult is the result of skills/list. All skills fit in one page,
// so nextCursor is never set.
type ListSkillsResult struct {
	mcp.ResultBase
	Skills     []Skill `json:"skills"`
	NextCursor string  `json:"nextCursor,omitempty"`
}

// GetSkillParams are the params of skills/get.
type GetSkillParams struct {
	mcp.ParamsBase
	URI string `json:"uri"`
}

// GetSkillResult is the result of skills/get.
type GetSkillResult struct {
	mcp.ResultBase
	Skill *Skill `json:"skill"`
}

// skillFile is a file of a skill, with its served URI and content.
type skillFile struct {
	uri  string
	data []byte
}

// skillSet is every published skill, loaded once from skillsFS.
type skillSet struct {
	skills []Skill          // sorted by name
	files  []skillFile      // every file of every skill
	byURI  map[string]Skill // SKILL.md URI -> skill
}

var publishedSkills = mustLoadSkills(skillsFS, "skills")

func mustLoadSkills(fsys fs.FS, root string) *skillSet {
	set, err := loadSkills(fsys, root)
	if err != nil {
		panic(fmt.Sprintf("skills: %v", err))
	}
	return set
}

// loadSkills reads each directory under root as a skill and computes its
// manifest from the bytes that will be served.
func loadSkills(fsys fs.FS, root string) (*skillSet, error) {
	entries, err := fs.ReadDir(fsys, root)
	if err != nil {
		return nil, err
	}
	set := &skillSet{byURI: map[string]Skill{}}
	for _, e := range entries {
		if !e.IsDir() {
			return nil, fmt.Errorf("%s/%s: only skill directories may be placed under %s", root, e.Name(), root)
		}
		name := e.Name()
		dir := path.Join(root, name)

		skillMD, err := fs.ReadFile(fsys, path.Join(dir, "SKILL.md"))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		fm, err := parseFrontmatter(skillMD)
		if err != nil {
			return nil, fmt.Errorf("%s/SKILL.md: %w", name, err)
		}
		if fm["name"] != name {
			return nil, fmt.Errorf("%s/SKILL.md: frontmatter name %q must match the directory name", name, fm["name"])
		}
		if fm["description"] == "" {
			return nil, fmt.Errorf("%s/SKILL.md: frontmatter description is required", name)
		}

		skill := Skill{URI: skillURIScheme + name + "/SKILL.md", Frontmatter: fm}
		err = fs.WalkDir(fsys, dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			data, err := fs.ReadFile(fsys, p)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(data)
			uri := skillURIScheme + name + "/" + strings.TrimPrefix(p, dir+"/")
			skill.Resources = append(skill.Resources, SkillResource{
				URI:    uri,
				Digest: "sha256:" + hex.EncodeToString(sum[:]),
				Size:   len(data),
			})
			set.files = append(set.files, skillFile{uri: uri, data: data})
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		// List SKILL.md first, then the supporting files in path order.
		sort.SliceStable(skill.Resources, func(i, j int) bool {
			if (skill.Resources[i].URI == skill.URI) != (skill.Resources[j].URI == skill.URI) {
				return skill.Resources[i].URI == skill.URI
			}
			return skill.Resources[i].URI < skill.Resources[j].URI
		})
		set.skills = append(set.skills, skill)
		set.byURI[skill.URI] = skill
	}
	return set, nil
}

// parseFrontmatter extracts the YAML frontmatter of a SKILL.md. Only flat
// "key: value" string fields are supported, which is all the published
// skills use; anything else is rejected so a richer frontmatter can't be
// silently dropped from skills/list.
func parseFrontmatter(md []byte) (map[string]string, error) {
	sc := bufio.NewScanner(strings.NewReader(string(md)))
	if !sc.Scan() || strings.TrimSpace(sc.Text()) != "---" {
		return nil, fmt.Errorf("missing frontmatter: the file must start with ---")
	}
	fm := map[string]string{}
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "---" {
			return fm, nil
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok || key == "" || strings.TrimSpace(key) != key {
			return nil, fmt.Errorf("unsupported frontmatter line %q: only flat key: value fields are supported", line)
		}
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, fmt.Errorf("frontmatter field %q has no value: nested or multi-line values are not supported", key)
		}
		// Keep the value a valid YAML plain scalar, since hosts parse the
		// frontmatter with a real YAML parser.
		if strings.Contains(value, ": ") || strings.Contains(value, " #") || strings.ContainsAny(value[:1], "\"'[]{}>|*&!%@`#,?:-") {
			return nil, fmt.Errorf("frontmatter field %q is not a plain YAML scalar: avoid \": \", \" #\" and leading indicator characters", key)
		}
		if _, dup := fm[key]; dup {
			return nil, fmt.Errorf("duplicate frontmatter field %q", key)
		}
		fm[key] = value
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("unterminated frontmatter: missing closing ---")
}

// skillFileMIMEType returns the MIME type a skill file is served with.
func skillFileMIMEType(uri string) string {
	if strings.HasSuffix(uri, ".md") {
		return "text/markdown"
	}
	return "text/plain"
}

// registerSkills publishes the embedded skills via the Skills extension:
// skills/list and skills/get return the manifests, and every file is a
// resource readable with resources/read.
func registerSkills(s *mcp.Server) {
	registerSkillSet(s, publishedSkills)
}

func registerSkillSet(s *mcp.Server, set *skillSet) {
	for _, f := range set.files {
		mimeType := skillFileMIMEType(f.uri)
		text := string(f.data)
		s.AddResource(&mcp.Resource{
			URI:      f.uri,
			Name:     strings.TrimPrefix(f.uri, skillURIScheme),
			MIMEType: mimeType,
			Size:     int64(len(f.data)),
		}, func(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{{
					URI:      req.Params.URI,
					MIMEType: mimeType,
					Text:     text,
				}},
			}, nil
		})
	}

	if err := mcp.AddReceivingCustomMethod(s, "skills/list",
		func(_ context.Context, _ *mcp.ServerSession, params *ListSkillsParams) (*ListSkillsResult, error) {
			if params != nil && params.Cursor != "" {
				return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: "invalid cursor"}
			}
			return &ListSkillsResult{Skills: set.skills}, nil
		}); err != nil {
		panic(err)
	}

	if err := mcp.AddReceivingCustomMethod(s, "skills/get",
		func(_ context.Context, _ *mcp.ServerSession, params *GetSkillParams) (*GetSkillResult, error) {
			if params == nil || params.URI == "" {
				return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: "uri is required"}
			}
			skill, ok := set.byURI[params.URI]
			if !ok {
				return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: fmt.Sprintf("unknown skill: %s", params.URI)}
			}
			return &GetSkillResult{Skill: &skill}, nil
		}); err != nil {
		panic(err)
	}
}
