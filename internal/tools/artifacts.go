package tools

import (
	"fmt"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// GetListArtifactsTool returns a tool that lists available artifacts in the current session.
func GetListArtifactsTool() tool.Tool {
	t, err := functiontool.New(
		functiontool.Config{
			Name:        "list_artifacts",
			Description: "Lists the names of all files (artifacts) available in the current session history, such as screenshots or documents.",
		},
		func(ctx tool.Context, _ struct{}) (map[string]any, error) {
			resp, err := ctx.Artifacts().List(ctx)
			if err != nil {
				return map[string]any{"error": fmt.Sprintf("failed to list artifacts: %v", err)}, nil
			}
			return map[string]any{"artifacts": resp.FileNames}, nil
		},
	)
	if err != nil {
		panic(err)
	}
	return t
}

// GetLoadArtifactTool returns a tool that loads the content of a specific artifact.
func GetLoadArtifactTool() tool.Tool {
	t, err := functiontool.New(
		functiontool.Config{
			Name:        "load_artifact",
			Description: "Loads the content of a specific artifact (file) by its name. Use this to 'see' screenshots or read file contents uploaded by the client.",
		},
		func(ctx tool.Context, args struct {
			Filename string `json:"filename" jsonschema:"The name of the artifact to load."`
			Name     string `json:"name,omitempty" jsonschema:"Alias for filename."`
			Artifact string `json:"artifact,omitempty" jsonschema:"Alias for filename."`
		}) (map[string]any, error) {
			target := args.Filename
			if target == "" {
				if args.Name != "" {
					target = args.Name
				} else if args.Artifact != "" {
					target = args.Artifact
				}
			}

			if target == "" {
				return map[string]any{"error": "missing required parameter: filename"}, nil
			}

			part, err := ctx.Artifacts().Load(ctx, target)
			if err != nil {
				return map[string]any{"error": fmt.Sprintf("failed to load artifact '%s': %v", target, err)}, nil
			}

			// ADK's Load returns a genai.Part. We return it so the framework can attach it to the tool response.
			// The LLM will receive the multimodal content (image/text/bytes).
			return map[string]any{
				"status":   "success",
				"filename": target,
				"content":  part.Part, // This is the genai.Part
			}, nil
		},
	)
	if err != nil {
		panic(err)
	}
	return t
}
