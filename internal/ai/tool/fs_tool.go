package tool

import (
	"encoding/json"
	"errors"
	"fmt"

	genkitAI "github.com/firebase/genkit/go/ai"
	genkitTool "github.com/firebase/genkit/go/ai/tool"
	"github.com/firebase/genkit/go/genkit"
)

type readLocalInput struct {
	Path string `json:"path" jsonschema_description:"Absolute or relative path to a file under an allowed directory."`
}

type writeLocalInput struct {
	Path    string `json:"path" jsonschema_description:"Absolute or relative path to write under an allowed directory."`
	Content string `json:"content" jsonschema_description:"Full file contents to write (overwrite)."`
}

type listLocalInput struct {
	Path string `json:"path" jsonschema_description:"Absolute or relative path to a directory under an allowed directory."`
}

func defineFSTools(g *genkit.Genkit) (read, write, list genkitAI.ToolRef) {
	read = genkit.DefineTool(g, ReadLocalFile,
		"Read a local file from the user's machine. Path must be under a configured allowed directory. Requires user approval.",
		func(tc *genkitAI.ToolContext, input readLocalInput) (string, error) {
			return runFS(tc, input.Path, func(deps Deps, abs string) (string, error) {
				return ReadFile(abs, deps.AllowedDirs, 0)
			})
		})

	write = genkit.DefineTool(g, WriteLocalFile,
		"Write (overwrite/create) a local file on the user's machine under an allowed directory. Requires user approval.",
		func(tc *genkitAI.ToolContext, input writeLocalInput) (string, error) {
			if len(input.Content) > defaultWriteMaxBytes {
				return "", genkitTool.Fail(tc, fmt.Errorf("content too large: %d bytes (max %d)", len(input.Content), defaultWriteMaxBytes))
			}
			return runFS(tc, input.Path, func(deps Deps, abs string) (string, error) {
				return WriteFile(abs, input.Content, deps.AllowedDirs)
			})
		})

	list = genkit.DefineTool(g, ListLocalDirectory,
		"List entries in a local directory under an allowed directory. Requires user approval.",
		func(tc *genkitAI.ToolContext, input listLocalInput) (string, error) {
			return runFS(tc, input.Path, func(deps Deps, abs string) (string, error) {
				res, err := ListDir(abs, deps.AllowedDirs, 0)
				if err != nil {
					return "", err
				}
				b, err := json.Marshal(res)
				if err != nil {
					return "", err
				}
				return string(b), nil
			})
		})
	return read, write, list
}

func runFS(
	tc *genkitAI.ToolContext,
	path string,
	exec func(Deps, string) (string, error),
) (string, error) {
	deps := FromCtx(tc)
	if !deps.EnableFS() {
		return "", genkitTool.Fail(tc, errors.New("local filesystem tools unavailable: no allowed directories configured"))
	}
	abs, err := ResolveUnderAllowlist(path, deps.AllowedDirs)
	if err != nil {
		return "", genkitTool.Fail(tc, err)
	}
	out, execErr := exec(deps, abs)
	if execErr != nil {
		return "", genkitTool.Fail(tc, execErr)
	}
	return out, nil
}
