package ai

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	genkitAI "github.com/firebase/genkit/go/ai"
)

const (
	useSkillToolName       = "use_skill"
	skillsPromptMarker     = "vela-skills-instructions"
	skillsMissingDescLabel = "No description provided."
)

// skillsMiddleware exposes enabled local SKILL.md directories to the model.
// SkillDirs are absolute paths to skill folders (each containing SKILL.md).
type skillsMiddleware struct {
	SkillDirs []string
}

func (s skillsMiddleware) Name() string { return "vela/skills" }

func (s skillsMiddleware) New(ctx context.Context) (*genkitAI.Hooks, error) {
	_ = ctx
	info := scanEnabledSkills(s.SkillDirs)
	if len(info) == 0 {
		return &genkitAI.Hooks{}, nil
	}

	useSkill := genkitAI.NewTool(
		useSkillToolName,
		"Use a skill by its name. Skills provide specialized instructions; no user approval is required.",
		func(_ *genkitAI.ToolContext, in struct {
			SkillName string `json:"skillName" jsonschema_description:"The name of the skill to use."`
		}) (string, error) {
			si, ok := info[in.SkillName]
			if !ok {
				return "", fmt.Errorf("skill %q not found or disabled", in.SkillName)
			}
			data, err := os.ReadFile(si.path)
			if err != nil {
				return "", fmt.Errorf("failed to read skill %q: %w", in.SkillName, err)
			}
			return string(data), nil
		},
	)

	prompt := buildEnabledSkillsPrompt(info)
	wrapGenerate := func(ctx context.Context, params *genkitAI.GenerateParams, next genkitAI.GenerateNext) (*genkitAI.ModelResponse, error) {
		params.Request = injectSkillsSystemPrompt(params.Request, prompt)
		return next(ctx, params)
	}

	return &genkitAI.Hooks{
		Tools:        []genkitAI.Tool{useSkill},
		WrapGenerate: wrapGenerate,
	}, nil
}

type enabledSkillInfo struct {
	path        string
	description string
}

func scanEnabledSkills(dirs []string) map[string]enabledSkillInfo {
	result := make(map[string]enabledSkillInfo)
	for _, dir := range dirs {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		abs, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		skillMD := filepath.Join(abs, skillMarkdownName)
		data, err := os.ReadFile(skillMD)
		if err != nil {
			continue
		}
		name := filepath.Base(abs)
		fm := parseSkillFrontmatter(data)
		desc := strings.TrimSpace(fm.Description)
		if desc == "" {
			desc = skillsMissingDescLabel
		}
		result[name] = enabledSkillInfo{path: skillMD, description: desc}
	}
	return result
}

func buildEnabledSkillsPrompt(info map[string]enabledSkillInfo) string {
	names := make([]string, 0, len(info))
	for name := range info {
		names = append(names, name)
	}
	sort.Strings(names)
	var b strings.Builder
	b.WriteString("<skills>\n")
	b.WriteString("You have access to a library of skills that serve as specialized instructions/personas.\n")
	b.WriteString("Strongly prefer to use them when working on anything related to them.\n")
	b.WriteString("Only use them once to load the context.\n")
	b.WriteString("Here are the available skills:\n")
	for _, name := range names {
		desc := info[name].description
		if desc == "" || desc == skillsMissingDescLabel {
			fmt.Fprintf(&b, " - %s\n", name)
			continue
		}
		fmt.Fprintf(&b, " - %s - %s\n", name, desc)
	}
	b.WriteString("</skills>")
	return b.String()
}

func injectSkillsSystemPrompt(req *genkitAI.ModelRequest, promptText string) *genkitAI.ModelRequest {
	if req == nil {
		return req
	}
	newReq := *req
	newReq.Messages = append([]*genkitAI.Message(nil), req.Messages...)

	for i, msg := range newReq.Messages {
		if msg == nil {
			continue
		}
		for j, part := range msg.Content {
			if !hasSkillsPromptMarker(part) {
				continue
			}
			if part.Text == promptText {
				return &newReq
			}
			msgCopy := msg.Clone()
			msgCopy.Content[j] = newSkillsPromptPart(promptText)
			newReq.Messages[i] = msgCopy
			return &newReq
		}
	}

	for i, msg := range newReq.Messages {
		if msg == nil || msg.Role != genkitAI.RoleSystem {
			continue
		}
		msgCopy := msg.Clone()
		msgCopy.Content = append(msgCopy.Content, newSkillsPromptPart(promptText))
		newReq.Messages[i] = msgCopy
		return &newReq
	}

	newReq.Messages = append(
		[]*genkitAI.Message{genkitAI.NewSystemMessage(newSkillsPromptPart(promptText))},
		newReq.Messages...,
	)
	return &newReq
}

func newSkillsPromptPart(text string) *genkitAI.Part {
	p := genkitAI.NewTextPart(text)
	p.Metadata = map[string]any{skillsPromptMarker: true}
	return p
}

func hasSkillsPromptMarker(p *genkitAI.Part) bool {
	if p == nil || !p.IsText() || p.Metadata == nil {
		return false
	}
	v, _ := p.Metadata[skillsPromptMarker].(bool)
	return v
}
