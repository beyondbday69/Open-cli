package prompt

import "github.com/opencode-ai/opencode/internal/llm/models"

func TitlePrompt(_ models.ModelProvider) string {
	return `You generate a concise, descriptive session title from the user's first message.

Rules:
- Maximum 50 characters
- Be specific and meaningful — capture the actual task or topic (e.g. "Fix auth JWT expiry bug", "Add CSV export to reports", "Explain Go generics")
- Use sentence case, no trailing punctuation
- No quotes, colons, or filler words like "Help with" or "Question about"
- Prefer action verbs when the request is a task (Fix, Add, Refactor, Write, Debug, Explain)
- Output exactly one line — the title only, nothing else`
}
