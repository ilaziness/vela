package ai

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	genkitAI "github.com/firebase/genkit/go/ai"
)

const (
	useSkillToolName       = "use_skill"
	skillsPromptMarker     = "vela-skills-instructions"
	skillsMissingDescLabel = "No description provided."
)

// skillsMiddleware exposes enabled local SKILL.md dirs from a catalog snapshot.
type skillsMiddleware struct {
	Skills []EnabledSkill
}

func (s skillsMiddleware) Name() string { return "vela/skills" }

func (s skillsMiddleware) New(ctx context.Context) (*genkitAI.Hooks, error) {
	_ = ctx
	info := skillsByName(s.Skills)
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
			data, err := os.ReadFile(skillMarkdownPath(si.Path))
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

func skillsByName(skills []EnabledSkill) map[string]EnabledSkill {
	result := make(map[string]EnabledSkill, len(skills))
	for _, s := range skills {
		name := strings.TrimSpace(s.Name)
		if name == "" || strings.TrimSpace(s.Path) == "" {
			continue
		}
		desc := strings.TrimSpace(s.Description)
		if desc == "" {
			desc = skillsMissingDescLabel
		}
		result[name] = EnabledSkill{Name: name, Description: desc, Path: s.Path}
	}
	return result
}

func buildEnabledSkillsPrompt(info map[string]EnabledSkill) string {
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
		desc := info[name].Description
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
